package vkbot

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestSendDocumentUploadsAndAttachesCSV(t *testing.T) {
	contents := []byte("дата;участники\n2026-10-24;2\n")
	var calls []string
	transport := roundTripFunc(func(r *http.Request) (*http.Response, error) {
		calls = append(calls, r.URL.Path)
		switch r.URL.Path {
		case "/method/docs.getMessagesUploadServer":
			if err := r.ParseForm(); err != nil {
				t.Fatal(err)
			}
			if r.Form.Get("peer_id") != "42" || r.Form.Get("type") != "doc" {
				t.Errorf("upload server request=%v", r.Form)
			}
			return jsonResponse(`{"response":{"upload_url":"https://vk.test/upload"}}`), nil
		case "/upload":
			if err := r.ParseMultipartForm(1 << 20); err != nil {
				t.Fatal(err)
			}
			file, header, err := r.FormFile("file")
			if err != nil {
				t.Fatal(err)
			}
			defer file.Close()
			got, err := io.ReadAll(file)
			if err != nil {
				t.Fatal(err)
			}
			if header.Filename != "stats.csv" || string(got) != string(contents) {
				t.Errorf("uploaded %s: %q", header.Filename, got)
			}
			return jsonResponse(`{"file":"uploaded-file"}`), nil
		case "/method/docs.save":
			if err := r.ParseForm(); err != nil {
				t.Fatal(err)
			}
			if r.Form.Get("file") != "uploaded-file" || r.Form.Get("title") != "stats.csv" {
				t.Errorf("save request=%v", r.Form)
			}
			return jsonResponse(`{"response":{"type":"doc","doc":{"owner_id":-123,"id":456,"access_key":"secret"}}}`), nil
		case "/method/messages.send":
			if err := r.ParseForm(); err != nil {
				t.Fatal(err)
			}
			if r.Form.Get("peer_id") != "42" || r.Form.Get("attachment") != "doc-123_456_secret" ||
				!strings.Contains(r.Form.Get("message"), "Выгрузка статистики") {
				t.Errorf("sent message=%v", r.Form)
			}
			return jsonResponse(`{"response":123}`), nil
		default:
			t.Errorf("unexpected request %s", r.URL.Path)
			return jsonResponse(`{"error":{"error_code":1,"error_msg":"unexpected"}}`), nil
		}
	})
	client := NewClient("test-token", 123, "https://vk.test/method", &http.Client{Transport: transport})
	if err := client.SendDocument(context.Background(), 42, "stats.csv", contents); err != nil {
		t.Fatal(err)
	}
	if len(calls) != 4 {
		t.Errorf("calls=%v", calls)
	}
}
