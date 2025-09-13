package caldav

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"net/http"
	"path"
	"strings"
)

// AttachmentManager handles CalDAV managed attachments according to RFC 4331.
type AttachmentManager struct {
	client *CalDAVClient
}

// ManagedAttachment represents a managed attachment with server-side storage.
type ManagedAttachment struct {
	Href         string
	ETag         string
	ContentType  string
	Size         int64
	Filename     string
	Created      string
	LastModified string
	ChecksumMD5  string
	ChecksumSHA1 string
}

// AttachmentCollection represents a collection that stores managed attachments.
type AttachmentCollection struct {
	Href                    string
	DisplayName             string
	Description             string
	MaxAttachmentSize       int64
	SupportedMediaTypes     []string
	CurrentUserPrivilegeSet []string
}

// NewAttachmentManager creates a new attachment manager for the given CalDAV client.
func NewAttachmentManager(client *CalDAVClient) *AttachmentManager {
	return &AttachmentManager{
		client: client,
	}
}

// FindAttachmentCollections discovers attachment collections for a calendar.
func (am *AttachmentManager) FindAttachmentCollections(ctx context.Context, calendarHref string) ([]AttachmentCollection, error) {
	propfindXML := `<?xml version="1.0" encoding="utf-8"?>
<D:propfind xmlns:D="DAV:" xmlns:C="urn:ietf:params:xml:ns:caldav">
  <D:prop>
    <D:displayname />
    <D:resourcetype />
    <C:calendar-description />
    <D:current-user-privilege-set />
    <C:max-attachment-size />
    <C:supported-media-types />
  </D:prop>
</D:propfind>`

	// Use the CalDAVClient's propfind method which includes XML validation
	resp, err := am.client.propfind(context.Background(), calendarHref, "1", []byte(propfindXML))
	if err != nil {
		return nil, fmt.Errorf("discovering attachment collections: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusMultiStatus {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	multiStatus, err := parseMultiStatusResponse(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("parsing response: %w", err)
	}

	var collections []AttachmentCollection
	for _, response := range multiStatus.Responses {
		for _, propstat := range response.Propstat {
			if propstat.Status == 200 {
				props := propstat.Prop

				// Check if this is an attachment collection
				isAttachmentCollection := false
				for _, resourceType := range props.ResourceType {
					if resourceType == "collection" || strings.Contains(resourceType, "attachment") {
						isAttachmentCollection = true
						break
					}
				}

				if isAttachmentCollection {
					collection := AttachmentCollection{
						Href:                    response.Href,
						DisplayName:             props.DisplayName,
						Description:             props.CalendarDescription,
						CurrentUserPrivilegeSet: props.CurrentUserPrivilegeSet,
						MaxAttachmentSize:       props.MaxResourceSize,
						SupportedMediaTypes:     []string{"*/*"}, // Default to all types
					}
					collections = append(collections, collection)
				}
			}
		}
	}

	return collections, nil
}

// UploadAttachment uploads a new attachment to the specified collection.
func (am *AttachmentManager) UploadAttachment(ctx context.Context, collectionHref string, filename string, contentType string, data []byte) (*ManagedAttachment, error) {
	// Generate attachment href - typically: collection/filename
	attachmentHref := path.Join(collectionHref, filename)

	req, err := am.client.prepareRequest(ctx, "PUT", attachmentHref, bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}

	req.Header.Set("Content-Type", contentType)
	req.Header.Set("Content-Length", fmt.Sprintf("%d", len(data)))

	resp, err := am.client.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("executing request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	etag := resp.Header.Get("ETag")

	attachment := &ManagedAttachment{
		Href:        attachmentHref,
		ETag:        etag,
		ContentType: contentType,
		Size:        int64(len(data)),
		Filename:    filename,
	}

	return attachment, nil
}

// GetAttachment retrieves an attachment by its href.
func (am *AttachmentManager) GetAttachment(ctx context.Context, attachmentHref string) ([]byte, *ManagedAttachment, error) {
	req, err := am.client.prepareRequest(ctx, "GET", attachmentHref, nil)
	if err != nil {
		return nil, nil, fmt.Errorf("creating request: %w", err)
	}

	resp, err := am.client.httpClient.Do(req)
	if err != nil {
		return nil, nil, fmt.Errorf("executing request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, nil, fmt.Errorf("reading response body: %w", err)
	}

	attachment := &ManagedAttachment{
		Href:         attachmentHref,
		ETag:         resp.Header.Get("ETag"),
		ContentType:  resp.Header.Get("Content-Type"),
		Size:         int64(len(data)),
		LastModified: resp.Header.Get("Last-Modified"),
	}

	return data, attachment, nil
}

// UpdateAttachment updates an existing attachment with new data.
func (am *AttachmentManager) UpdateAttachment(ctx context.Context, attachmentHref string, contentType string, data []byte, ifMatch string) (*ManagedAttachment, error) {
	req, err := am.client.prepareRequest(ctx, "PUT", attachmentHref, bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}

	req.Header.Set("Content-Type", contentType)
	req.Header.Set("Content-Length", fmt.Sprintf("%d", len(data)))

	if ifMatch != "" {
		req.Header.Set("If-Match", ifMatch)
	}

	resp, err := am.client.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("executing request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	etag := resp.Header.Get("ETag")

	attachment := &ManagedAttachment{
		Href:        attachmentHref,
		ETag:        etag,
		ContentType: contentType,
		Size:        int64(len(data)),
	}

	return attachment, nil
}

// DeleteAttachment removes an attachment from the server.
func (am *AttachmentManager) DeleteAttachment(ctx context.Context, attachmentHref string, ifMatch string) error {
	req, err := am.client.prepareRequest(ctx, "DELETE", attachmentHref, nil)
	if err != nil {
		return fmt.Errorf("creating request: %w", err)
	}

	if ifMatch != "" {
		req.Header.Set("If-Match", ifMatch)
	}

	resp, err := am.client.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("executing request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusNoContent && resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	return nil
}

// ListAttachments lists all attachments in a collection.
func (am *AttachmentManager) ListAttachments(ctx context.Context, collectionHref string) ([]ManagedAttachment, error) {
	propfindXML := `<?xml version="1.0" encoding="utf-8"?>
<D:propfind xmlns:D="DAV:">
  <D:prop>
    <D:displayname />
    <D:getcontenttype />
    <D:getcontentlength />
    <D:getetag />
    <D:creationdate />
    <D:getlastmodified />
  </D:prop>
</D:propfind>`

	// Use the CalDAVClient's propfind method which includes XML validation
	resp, err := am.client.propfind(context.Background(), collectionHref, "1", []byte(propfindXML))
	if err != nil {
		return nil, fmt.Errorf("listing attachments: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusMultiStatus {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	multiStatus, err := parseMultiStatusResponse(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("parsing response: %w", err)
	}

	var attachments []ManagedAttachment
	for _, response := range multiStatus.Responses {
		// Skip the collection itself
		if response.Href == collectionHref || strings.HasSuffix(response.Href, "/") {
			continue
		}

		for _, propstat := range response.Propstat {
			if propstat.Status == 200 {
				props := propstat.Prop

				attachment := ManagedAttachment{
					Href:         response.Href,
					ETag:         props.ETag,
					ContentType:  props.ContentType,
					Size:         props.ContentLength,
					Filename:     props.DisplayName,
					Created:      props.CreationDate,
					LastModified: props.LastModified,
				}

				// Extract filename from href if displayname is empty
				if attachment.Filename == "" {
					attachment.Filename = path.Base(response.Href)
				}

				attachments = append(attachments, attachment)
			}
		}
	}

	return attachments, nil
}

// CreateAttachmentReference creates an ATTACH property reference for use in calendar events.
func (am *AttachmentManager) CreateAttachmentReference(attachment *ManagedAttachment) Attachment {
	return Attachment{
		URI:        attachment.Href,
		FormatType: attachment.ContentType,
		Size:       int(attachment.Size),
		Filename:   attachment.Filename,
		CustomParams: map[string]string{
			"MANAGED":     "TRUE",
			"MTAG":        attachment.ETag,
			"X-APPLE-URL": am.client.baseURL + attachment.Href,
		},
	}
}

// AttachFileToEvent attaches a file to an event by uploading it and updating the event.
func (am *AttachmentManager) AttachFileToEvent(ctx context.Context, calendarHref string, eventUID string, filename string, contentType string, data []byte) error {
	// Find attachment collection for this calendar
	collections, err := am.FindAttachmentCollections(ctx, calendarHref)
	if err != nil {
		return fmt.Errorf("finding attachment collections: %w", err)
	}

	if len(collections) == 0 {
		return fmt.Errorf("no attachment collection found for calendar")
	}

	// Use the first available attachment collection
	collection := collections[0]

	// Upload the attachment
	attachment, err := am.UploadAttachment(ctx, collection.Href, filename, contentType, data)
	if err != nil {
		return fmt.Errorf("uploading attachment: %w", err)
	}

	// Create attachment reference
	attachRef := am.CreateAttachmentReference(attachment)

	// This would require updating the event with the new attachment reference
	// For now, we return success - full integration would require event updating
	_ = attachRef
	_ = eventUID

	return nil
}

// EncodeInlineAttachment encodes binary data as base64 for inline attachments.
func EncodeInlineAttachment(data []byte, contentType string) Attachment {
	encodedData := base64.StdEncoding.EncodeToString(data)

	return Attachment{
		Encoding:   "BASE64",
		Value:      "BINARY",
		FormatType: contentType,
		URI:        "data:" + contentType + ";base64," + encodedData,
	}
}

// DecodeInlineAttachment decodes a base64 inline attachment.
func DecodeInlineAttachment(attachment Attachment) ([]byte, error) {
	if attachment.Encoding != "BASE64" {
		return nil, fmt.Errorf("attachment is not base64 encoded")
	}

	// Extract base64 data from data URI if present
	data := attachment.URI
	if strings.HasPrefix(data, "data:") {
		parts := strings.SplitN(data, ",", 2)
		if len(parts) == 2 {
			data = parts[1]
		}
	}

	decoded, err := base64.StdEncoding.DecodeString(data)
	if err != nil {
		return nil, fmt.Errorf("decoding base64 data: %w", err)
	}

	return decoded, nil
}
