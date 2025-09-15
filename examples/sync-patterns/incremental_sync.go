//go:build ignore
// +build ignore

package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"time"

	caldav "github.com/kevmarchant/go-icloud-caldav"
)

const tokenStorageFile = "sync_tokens.json"

type SyncTokenStorage struct {
	Tokens   map[string]string `json:"tokens"`
	LastSync time.Time         `json:"last_sync"`
}

func loadSyncTokens() (*SyncTokenStorage, error) {
	storage := &SyncTokenStorage{
		Tokens: make(map[string]string),
	}

	data, err := ioutil.ReadFile(tokenStorageFile)
	if err != nil {
		if os.IsNotExist(err) {
			return storage, nil
		}
		return nil, err
	}

	if err := json.Unmarshal(data, storage); err != nil {
		return nil, err
	}

	return storage, nil
}

func saveSyncTokens(storage *SyncTokenStorage) error {
	storage.LastSync = time.Now()

	data, err := json.MarshalIndent(storage, "", "  ")
	if err != nil {
		return err
	}

	return ioutil.WriteFile(tokenStorageFile, data, 0644)
}

func main() {
	username := os.Getenv("ICLOUD_USERNAME")
	password := os.Getenv("ICLOUD_PASSWORD")

	if username == "" || password == "" {
		log.Fatal("Please set ICLOUD_USERNAME and ICLOUD_PASSWORD environment variables")
	}

	client := caldav.NewClient(username, password)

	ctx := context.Background()

	storage, err := loadSyncTokens()
	if err != nil {
		log.Fatalf("Failed to load sync tokens: %v", err)
	}

	if len(storage.Tokens) == 0 {
		fmt.Println("Performing initial sync...")
		if err := performInitialSync(ctx, client, storage); err != nil {
			log.Fatalf("Initial sync failed: %v", err)
		}
	} else {
		fmt.Printf("Performing incremental sync (last sync: %v)...\n", storage.LastSync)
		if err := performIncrementalSync(ctx, client, storage); err != nil {
			log.Fatalf("Incremental sync failed: %v", err)
		}
	}

	if err := saveSyncTokens(storage); err != nil {
		log.Fatalf("Failed to save sync tokens: %v", err)
	}

	fmt.Println("Sync completed successfully!")
}

func performInitialSync(ctx context.Context, client *caldav.CalDAVClient, storage *SyncTokenStorage) error {
	principal, err := client.FindCurrentUserPrincipal(ctx)
	if err != nil {
		return fmt.Errorf("finding principal: %w", err)
	}

	calendarHomeSet, err := client.FindCalendarHomeSet(ctx, principal)
	if err != nil {
		return fmt.Errorf("finding calendar home: %w", err)
	}

	calendars, err := client.FindCalendars(ctx, calendarHomeSet)
	if err != nil {
		return fmt.Errorf("finding calendars: %w", err)
	}

	fmt.Printf("Found %d calendars\n", len(calendars))

	totalEvents := 0
	for _, cal := range calendars {
		fmt.Printf("\nSyncing calendar: %s\n", cal.DisplayName)

		syncResp, err := client.InitialSync(ctx, cal.Href)
		if err != nil {
			fmt.Printf("  Error: %v\n", err)
			continue
		}

		storage.Tokens[cal.Href] = syncResp.SyncToken

		newItems := syncResp.GetNewItems()
		fmt.Printf("  Found %d events\n", len(newItems))
		totalEvents += len(newItems)

		for i, item := range newItems {
			if i < 3 {
				fmt.Printf("    - %s\n", item.Href)
			} else if i == 3 {
				fmt.Printf("    ... and %d more\n", len(newItems)-3)
				break
			}
		}
	}

	fmt.Printf("\nTotal events synced: %d\n", totalEvents)
	return nil
}

func performIncrementalSync(ctx context.Context, client *caldav.CalDAVClient, storage *SyncTokenStorage) error {
	principal, err := client.FindCurrentUserPrincipal(ctx)
	if err != nil {
		return fmt.Errorf("finding principal: %w", err)
	}

	calendarHomeSet, err := client.FindCalendarHomeSet(ctx, principal)
	if err != nil {
		return fmt.Errorf("finding calendar home: %w", err)
	}

	calendars, err := client.FindCalendars(ctx, calendarHomeSet)
	if err != nil {
		return fmt.Errorf("finding calendars: %w", err)
	}

	totalNew := 0
	totalModified := 0
	totalDeleted := 0

	for _, cal := range calendars {
		token, hasToken := storage.Tokens[cal.Href]
		if !hasToken {
			fmt.Printf("\nNo sync token for %s, performing initial sync...\n", cal.DisplayName)

			syncResp, err := client.InitialSync(ctx, cal.Href)
			if err != nil {
				fmt.Printf("  Error: %v\n", err)
				continue
			}

			storage.Tokens[cal.Href] = syncResp.SyncToken
			fmt.Printf("  Found %d events\n", len(syncResp.Changes))
			totalNew += len(syncResp.Changes)
			continue
		}

		syncResp, err := client.IncrementalSync(ctx, cal.Href, token)
		if err != nil {
			fmt.Printf("\nSync token invalid for %s, performing full sync...\n", cal.DisplayName)

			syncResp, err = client.InitialSync(ctx, cal.Href)
			if err != nil {
				fmt.Printf("  Error: %v\n", err)
				continue
			}

			storage.Tokens[cal.Href] = syncResp.SyncToken
			fmt.Printf("  Found %d events\n", len(syncResp.Changes))
			totalNew += len(syncResp.Changes)
			continue
		}

		storage.Tokens[cal.Href] = syncResp.SyncToken

		if !syncResp.HasChanges() {
			continue
		}

		fmt.Printf("\nChanges in %s:\n", cal.DisplayName)

		newItems := syncResp.GetNewItems()
		if len(newItems) > 0 {
			fmt.Printf("  New events: %d\n", len(newItems))
			totalNew += len(newItems)
			for i, item := range newItems {
				if i < 3 {
					fmt.Printf("    + %s\n", item.Href)
				} else if i == 3 {
					fmt.Printf("    ... and %d more\n", len(newItems)-3)
					break
				}
			}
		}

		modifiedItems := syncResp.GetModifiedItems()
		if len(modifiedItems) > 0 {
			fmt.Printf("  Modified events: %d\n", len(modifiedItems))
			totalModified += len(modifiedItems)
			for i, item := range modifiedItems {
				if i < 3 {
					fmt.Printf("    * %s\n", item.Href)
				} else if i == 3 {
					fmt.Printf("    ... and %d more\n", len(modifiedItems)-3)
					break
				}
			}
		}

		deletedItems := syncResp.GetDeletedItems()
		if len(deletedItems) > 0 {
			fmt.Printf("  Deleted events: %d\n", len(deletedItems))
			totalDeleted += len(deletedItems)
			for i, item := range deletedItems {
				if i < 3 {
					fmt.Printf("    - %s\n", item.Href)
				} else if i == 3 {
					fmt.Printf("    ... and %d more\n", len(deletedItems)-3)
					break
				}
			}
		}
	}

	fmt.Printf("\nSync Summary:\n")
	fmt.Printf("  New: %d\n", totalNew)
	fmt.Printf("  Modified: %d\n", totalModified)
	fmt.Printf("  Deleted: %d\n", totalDeleted)

	if totalNew == 0 && totalModified == 0 && totalDeleted == 0 {
		fmt.Println("  No changes since last sync")
	}

	return nil
}

func performIncrementalSyncWithBatch(ctx context.Context, client *caldav.CalDAVClient, storage *SyncTokenStorage) error {
	results, err := client.SyncAllCalendars(ctx, storage.Tokens)
	if err != nil {
		return fmt.Errorf("sync all calendars: %w", err)
	}

	totalNew := 0
	totalModified := 0
	totalDeleted := 0

	for calendarURL, syncResp := range results {
		storage.Tokens[calendarURL] = syncResp.SyncToken

		if !syncResp.HasChanges() {
			continue
		}

		totalNew += len(syncResp.GetNewItems())
		totalModified += len(syncResp.GetModifiedItems())
		totalDeleted += len(syncResp.GetDeletedItems())
	}

	fmt.Printf("\nBatch Sync Summary:\n")
	fmt.Printf("  Calendars synced: %d\n", len(results))
	fmt.Printf("  New: %d\n", totalNew)
	fmt.Printf("  Modified: %d\n", totalModified)
	fmt.Printf("  Deleted: %d\n", totalDeleted)

	return nil
}
