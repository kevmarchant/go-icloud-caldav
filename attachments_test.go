package caldav

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestNewAttachmentManager(t *testing.T) {
	client := NewClient("test@example.com", "password")
	manager := NewAttachmentManager(client)

	if manager.client != client {
		t.Errorf("expected client to be set correctly")
	}
}

func TestAttachmentManager_FindAttachmentCollections(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "PROPFIND" {
			t.Errorf("expected PROPFIND request, got %s", r.Method)
		}

		if r.Header.Get("Depth") != "1" {
			t.Errorf("expected Depth: 1, got %s", r.Header.Get("Depth"))
		}

		response := `<?xml version="1.0" encoding="utf-8"?>
<D:multistatus xmlns:D="DAV:" xmlns:C="urn:ietf:params:xml:ns:caldav">
  <D:response>
    <D:href>/calendars/test/attachments/</D:href>
    <D:propstat>
      <D:prop>
        <D:displayname>Attachments</D:displayname>
        <D:resourcetype>
          <D:collection />
          <C:attachment-collection />
        </D:resourcetype>
        <C:calendar-description>Attachment storage</C:calendar-description>
        <D:current-user-privilege-set>
          <D:privilege><D:read /></D:privilege>
          <D:privilege><D:write /></D:privilege>
        </D:current-user-privilege-set>
        <C:max-attachment-size>10485760</C:max-attachment-size>
      </D:prop>
      <D:status>HTTP/1.1 200 OK</D:status>
    </D:propstat>
  </D:response>
</D:multistatus>`

		w.Header().Set("Content-Type", "text/xml")
		w.WriteHeader(http.StatusMultiStatus)
		_, _ = w.Write([]byte(response))
	}))
	defer server.Close()

	client := NewClient("test", "test")
	client.baseURL = server.URL

	manager := NewAttachmentManager(client)
	collections, err := manager.FindAttachmentCollections(context.Background(), "/calendars/test/")
	if err != nil {
		t.Fatalf("FindAttachmentCollections failed: %v", err)
	}

	if len(collections) != 1 {
		t.Errorf("expected 1 collection, got %d", len(collections))
	}

	collection := collections[0]
	if collection.Href != "/calendars/test/attachments/" {
		t.Errorf("expected href /calendars/test/attachments/, got %s", collection.Href)
	}

	if collection.DisplayName != "Attachments" {
		t.Errorf("expected display name 'Attachments', got %s", collection.DisplayName)
	}

	if collection.Description != "Attachment storage" {
		t.Errorf("expected description 'Attachment storage', got %s", collection.Description)
	}
}

func TestAttachmentManager_UploadAttachment(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "PUT" {
			t.Errorf("expected PUT request, got %s", r.Method)
		}

		if r.Header.Get("Content-Type") != "text/plain" {
			t.Errorf("expected Content-Type text/plain, got %s", r.Header.Get("Content-Type"))
		}

		w.Header().Set("ETag", "\"12345\"")
		w.WriteHeader(http.StatusCreated)
	}))
	defer server.Close()

	client := NewClient("test", "test")
	client.baseURL = server.URL

	manager := NewAttachmentManager(client)
	data := []byte("test file content")
	attachment, err := manager.UploadAttachment(context.Background(), "/attachments/", "test.txt", "text/plain", data)
	if err != nil {
		t.Fatalf("UploadAttachment failed: %v", err)
	}

	if attachment.ETag != "\"12345\"" {
		t.Errorf("expected ETag \"12345\", got %s", attachment.ETag)
	}

	if attachment.ContentType != "text/plain" {
		t.Errorf("expected ContentType text/plain, got %s", attachment.ContentType)
	}

	if attachment.Size != int64(len(data)) {
		t.Errorf("expected size %d, got %d", len(data), attachment.Size)
	}

	if attachment.Filename != "test.txt" {
		t.Errorf("expected filename test.txt, got %s", attachment.Filename)
	}
}

func TestAttachmentManager_GetAttachment(t *testing.T) {
	testContent := "test file content"

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" {
			t.Errorf("expected GET request, got %s", r.Method)
		}

		w.Header().Set("Content-Type", "text/plain")
		w.Header().Set("ETag", "\"12345\"")
		w.Header().Set("Last-Modified", "Wed, 01 Jan 2024 12:00:00 GMT")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(testContent))
	}))
	defer server.Close()

	client := NewClient("test", "test")
	client.baseURL = server.URL

	manager := NewAttachmentManager(client)
	data, attachment, err := manager.GetAttachment(context.Background(), "/attachments/test.txt")
	if err != nil {
		t.Fatalf("GetAttachment failed: %v", err)
	}

	if string(data) != testContent {
		t.Errorf("expected content %s, got %s", testContent, string(data))
	}

	if attachment.ETag != "\"12345\"" {
		t.Errorf("expected ETag \"12345\", got %s", attachment.ETag)
	}

	if attachment.ContentType != "text/plain" {
		t.Errorf("expected ContentType text/plain, got %s", attachment.ContentType)
	}

	if attachment.LastModified != "Wed, 01 Jan 2024 12:00:00 GMT" {
		t.Errorf("expected LastModified Wed, 01 Jan 2024 12:00:00 GMT, got %s", attachment.LastModified)
	}
}

func TestAttachmentManager_UpdateAttachment(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "PUT" {
			t.Errorf("expected PUT request, got %s", r.Method)
		}

		if r.Header.Get("If-Match") != "\"12345\"" {
			t.Errorf("expected If-Match \"12345\", got %s", r.Header.Get("If-Match"))
		}

		w.Header().Set("ETag", "\"12346\"")
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := NewClient("test", "test")
	client.baseURL = server.URL

	manager := NewAttachmentManager(client)
	data := []byte("updated content")
	attachment, err := manager.UpdateAttachment(context.Background(), "/attachments/test.txt", "text/plain", data, "\"12345\"")
	if err != nil {
		t.Fatalf("UpdateAttachment failed: %v", err)
	}

	if attachment.ETag != "\"12346\"" {
		t.Errorf("expected ETag \"12346\", got %s", attachment.ETag)
	}
}

func TestAttachmentManager_DeleteAttachment(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "DELETE" {
			t.Errorf("expected DELETE request, got %s", r.Method)
		}

		if r.Header.Get("If-Match") != "\"12345\"" {
			t.Errorf("expected If-Match \"12345\", got %s", r.Header.Get("If-Match"))
		}

		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	client := NewClient("test", "test")
	client.baseURL = server.URL

	manager := NewAttachmentManager(client)
	err := manager.DeleteAttachment(context.Background(), "/attachments/test.txt", "\"12345\"")
	if err != nil {
		t.Fatalf("DeleteAttachment failed: %v", err)
	}
}

func TestAttachmentManager_ListAttachments(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "PROPFIND" {
			t.Errorf("expected PROPFIND request, got %s", r.Method)
		}

		response := `<?xml version="1.0" encoding="utf-8"?>
<D:multistatus xmlns:D="DAV:">
  <D:response>
    <D:href>/attachments/</D:href>
    <D:propstat>
      <D:prop>
        <D:displayname>Attachments</D:displayname>
        <D:resourcetype><D:collection /></D:resourcetype>
      </D:prop>
      <D:status>HTTP/1.1 200 OK</D:status>
    </D:propstat>
  </D:response>
  <D:response>
    <D:href>/attachments/test.txt</D:href>
    <D:propstat>
      <D:prop>
        <D:displayname>test.txt</D:displayname>
        <D:getcontenttype>text/plain</D:getcontenttype>
        <D:getcontentlength>17</D:getcontentlength>
        <D:getetag>"12345"</D:getetag>
        <D:creationdate>2024-01-01T12:00:00Z</D:creationdate>
        <D:getlastmodified>Wed, 01 Jan 2024 12:00:00 GMT</D:getlastmodified>
      </D:prop>
      <D:status>HTTP/1.1 200 OK</D:status>
    </D:propstat>
  </D:response>
  <D:response>
    <D:href>/attachments/image.jpg</D:href>
    <D:propstat>
      <D:prop>
        <D:displayname>image.jpg</D:displayname>
        <D:getcontenttype>image/jpeg</D:getcontenttype>
        <D:getcontentlength>51200</D:getcontentlength>
        <D:getetag>"67890"</D:getetag>
        <D:creationdate>2024-01-01T13:00:00Z</D:creationdate>
        <D:getlastmodified>Wed, 01 Jan 2024 13:00:00 GMT</D:getlastmodified>
      </D:prop>
      <D:status>HTTP/1.1 200 OK</D:status>
    </D:propstat>
  </D:response>
</D:multistatus>`

		w.Header().Set("Content-Type", "text/xml")
		w.WriteHeader(http.StatusMultiStatus)
		_, _ = w.Write([]byte(response))
	}))
	defer server.Close()

	client := NewClient("test", "test")
	client.baseURL = server.URL

	manager := NewAttachmentManager(client)
	attachments, err := manager.ListAttachments(context.Background(), "/attachments/")
	if err != nil {
		t.Fatalf("ListAttachments failed: %v", err)
	}

	if len(attachments) != 2 {
		t.Errorf("expected 2 attachments, got %d", len(attachments))
	}

	// Check first attachment
	attachment1 := attachments[0]
	if attachment1.Href != "/attachments/test.txt" {
		t.Errorf("expected href /attachments/test.txt, got %s", attachment1.Href)
	}

	if attachment1.ContentType != "text/plain" {
		t.Errorf("expected ContentType text/plain, got %s", attachment1.ContentType)
	}

	if attachment1.Size != 17 {
		t.Errorf("expected size 17, got %d", attachment1.Size)
	}

	// Check second attachment
	attachment2 := attachments[1]
	if attachment2.Href != "/attachments/image.jpg" {
		t.Errorf("expected href /attachments/image.jpg, got %s", attachment2.Href)
	}

	if attachment2.ContentType != "image/jpeg" {
		t.Errorf("expected ContentType image/jpeg, got %s", attachment2.ContentType)
	}

	if attachment2.Size != 51200 {
		t.Errorf("expected size 51200, got %d", attachment2.Size)
	}
}

func TestAttachmentManager_CreateAttachmentReference(t *testing.T) {
	client := NewClient("test@example.com", "password")
	manager := NewAttachmentManager(client)

	attachment := &ManagedAttachment{
		Href:        "/attachments/test.txt",
		ETag:        "\"12345\"",
		ContentType: "text/plain",
		Size:        17,
		Filename:    "test.txt",
	}

	ref := manager.CreateAttachmentReference(attachment)

	if ref.URI != "/attachments/test.txt" {
		t.Errorf("expected URI /attachments/test.txt, got %s", ref.URI)
	}

	if ref.FormatType != "text/plain" {
		t.Errorf("expected FormatType text/plain, got %s", ref.FormatType)
	}

	if ref.Size != 17 {
		t.Errorf("expected Size 17, got %d", ref.Size)
	}

	if ref.Filename != "test.txt" {
		t.Errorf("expected Filename test.txt, got %s", ref.Filename)
	}

	if ref.CustomParams["MANAGED"] != "TRUE" {
		t.Errorf("expected MANAGED parameter to be TRUE, got %s", ref.CustomParams["MANAGED"])
	}

	if ref.CustomParams["MTAG"] != "\"12345\"" {
		t.Errorf("expected MTAG parameter to be \"12345\", got %s", ref.CustomParams["MTAG"])
	}
}

func TestEncodeInlineAttachment(t *testing.T) {
	data := []byte("Hello, World!")
	contentType := "text/plain"

	attachment := EncodeInlineAttachment(data, contentType)

	if attachment.Encoding != "BASE64" {
		t.Errorf("expected Encoding BASE64, got %s", attachment.Encoding)
	}

	if attachment.Value != "BINARY" {
		t.Errorf("expected Value BINARY, got %s", attachment.Value)
	}

	if attachment.FormatType != contentType {
		t.Errorf("expected FormatType %s, got %s", contentType, attachment.FormatType)
	}

	if !strings.HasPrefix(attachment.URI, "data:text/plain;base64,") {
		t.Errorf("expected URI to start with data:text/plain;base64, got %s", attachment.URI)
	}
}

func TestDecodeInlineAttachment(t *testing.T) {
	originalData := []byte("Hello, World!")
	attachment := EncodeInlineAttachment(originalData, "text/plain")

	decodedData, err := DecodeInlineAttachment(attachment)
	if err != nil {
		t.Fatalf("DecodeInlineAttachment failed: %v", err)
	}

	if string(decodedData) != string(originalData) {
		t.Errorf("expected decoded data %s, got %s", string(originalData), string(decodedData))
	}
}

func TestDecodeInlineAttachment_InvalidEncoding(t *testing.T) {
	attachment := Attachment{
		Encoding: "QUOTED-PRINTABLE",
		URI:      "invalid",
	}

	_, err := DecodeInlineAttachment(attachment)
	if err == nil {
		t.Errorf("expected error for non-base64 attachment")
	}
}

func TestAttachmentStructures(t *testing.T) {
	// Test ManagedAttachment structure
	attachment := ManagedAttachment{
		Href:         "/attachments/test.txt",
		ETag:         "\"12345\"",
		ContentType:  "text/plain",
		Size:         17,
		Filename:     "test.txt",
		Created:      "2024-01-01T12:00:00Z",
		LastModified: "Wed, 01 Jan 2024 12:00:00 GMT",
		ChecksumMD5:  "d41d8cd98f00b204e9800998ecf8427e",
		ChecksumSHA1: "da39a3ee5e6b4b0d3255bfef95601890afd80709",
	}

	if attachment.Href != "/attachments/test.txt" {
		t.Errorf("expected href /attachments/test.txt, got %s", attachment.Href)
	}

	// Test AttachmentCollection structure
	collection := AttachmentCollection{
		Href:                    "/attachments/",
		DisplayName:             "Attachments",
		Description:             "Attachment storage",
		MaxAttachmentSize:       10485760,
		SupportedMediaTypes:     []string{"text/*", "image/*"},
		CurrentUserPrivilegeSet: []string{"read", "write"},
	}

	if collection.MaxAttachmentSize != 10485760 {
		t.Errorf("expected MaxAttachmentSize 10485760, got %d", collection.MaxAttachmentSize)
	}

	if len(collection.SupportedMediaTypes) != 2 {
		t.Errorf("expected 2 supported media types, got %d", len(collection.SupportedMediaTypes))
	}
}

func TestAttachmentManager_AttachFileToEvent(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/calendars/test/":
			// PROPFIND for attachment collections
			response := `<?xml version="1.0" encoding="utf-8"?>
<D:multistatus xmlns:D="DAV:" xmlns:C="urn:ietf:params:xml:ns:caldav">
  <D:response>
    <D:href>/attachments/</D:href>
    <D:propstat>
      <D:prop>
        <D:displayname>Attachments</D:displayname>
        <D:resourcetype><D:collection /></D:resourcetype>
      </D:prop>
      <D:status>HTTP/1.1 200 OK</D:status>
    </D:propstat>
  </D:response>
</D:multistatus>`
			w.Header().Set("Content-Type", "text/xml")
			w.WriteHeader(http.StatusMultiStatus)
			_, _ = w.Write([]byte(response))

		case "/attachments/test.txt":
			// PUT for attachment upload
			w.Header().Set("ETag", "\"12345\"")
			w.WriteHeader(http.StatusCreated)

		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	client := NewClient("test", "test")
	client.baseURL = server.URL

	manager := NewAttachmentManager(client)
	data := []byte("test file content")
	err := manager.AttachFileToEvent(context.Background(), "/calendars/test/", "event-uid-123", "test.txt", "text/plain", data)
	if err != nil {
		t.Fatalf("AttachFileToEvent failed: %v", err)
	}
}
