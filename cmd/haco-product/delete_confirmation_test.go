package main

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/SLktEx/Hacocoon/internal/controlapi"
	"github.com/SLktEx/Hacocoon/internal/core"
	"github.com/SLktEx/Hacocoon/modules/plugin/oci"
	"github.com/SLktEx/Hacocoon/modules/standard/gitrepo"
)

type confirmationWriter struct{ remaining int }

type completionFailureWriter struct{ client *unusedReviewClient }

func (w completionFailureWriter) Write(p []byte) (int, error) {
	if len(w.client.deleted) != 0 {
		return 0, io.ErrClosedPipe
	}
	return len(p), nil
}

func (w *confirmationWriter) Write(p []byte) (int, error) {
	if w.remaining == 0 {
		return 0, io.ErrClosedPipe
	}
	w.remaining--
	return len(p), nil
}

type unexpectedConfirmationRead struct{ t *testing.T }

func (r unexpectedConfirmationRead) Read([]byte) (int, error) {
	r.t.Fatal("read input after failed display or explicit --yes")
	return 0, io.EOF
}

func TestDataDeletionLanguageDoesNotChangeAnswers(t *testing.T) {
	for _, language := range []string{"en", "ja"} {
		t.Run(language, func(t *testing.T) {
			t.Setenv("HACO_UI_LANGUAGE", language)
			for _, kind := range []string{"workspace", "source", "store", "base", "image"} {
				for _, input := range []string{"y\n", "yes\n", " YES \n", "\n", "N\n", "no\n", "はい\n", "yes", strings.Repeat(" ", 128) + "yes\n"} {
					var output bytes.Buffer
					code := confirmDataDeletion(strings.NewReader(input), &output, false, kind+".delete_warning", kind+".delete_prompt", kind+".retained")
					allowed := input == "y\n" || input == "yes\n" || input == " YES \n"
					if (code == 0) != allowed {
						t.Fatalf("%s %q: code=%d", kind, input, code)
					}
					want := "[y/N]"
					if language == "ja" {
						want = "未入力は保持"
					}
					if !strings.Contains(output.String(), want) || strings.Contains(output.String(), kind+".delete_") {
						t.Fatalf("untranslated confirmation: %s", output.String())
					}
				}
			}
		})
	}
}

func TestDataDeletionRefusesDisplayFailureBeforeReading(t *testing.T) {
	for _, language := range []string{"en", "ja"} {
		t.Run(language, func(t *testing.T) {
			t.Setenv("HACO_UI_LANGUAGE", language)
			for _, yes := range []bool{false, true} {
				for _, remaining := range []int{0, 1} {
					code := confirmDataDeletion(unexpectedConfirmationRead{t}, &confirmationWriter{remaining}, yes, "source.delete_warning", "source.delete_prompt", "source.retained")
					want := 1
					if yes && remaining == 1 {
						want = 0
					}
					if code != want {
						t.Fatalf("yes=%t write=%d code=%d want=%d", yes, remaining, code, want)
					}
				}
			}
		})
	}
}

func TestDataDeletionRejectsPipeEvenWithYesText(t *testing.T) {
	in, writer, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer in.Close()
	if _, err := writer.WriteString("yes\n"); err != nil {
		t.Fatal(err)
	}
	writer.Close()
	var output bytes.Buffer
	if code := confirmDataDeletion(in, &output, false, "source.delete_warning", "source.delete_prompt", "source.retained"); code != 2 {
		t.Fatalf("pipe authorized deletion: code=%d", code)
	}
}

func TestSourceDeletionLocalePreservesRequestJSONAndRawFailure(t *testing.T) {
	var englishJSON string
	var englishRequests []controlapi.RepositoryManageRequest
	for _, language := range []string{"en", "ja"} {
		t.Run(language, func(t *testing.T) {
			t.Setenv("HACO_UI_LANGUAGE", language)
			object := gitrepo.Object{Kind: "repo", ID: "source", Owner: strings.Repeat("a", 32), State: "ready", Remote: "https://example.test/repo.git", Branch: "main"}
			c := &sourceClientFake{all: controlapi.RepositoryManageResponse{Sources: []gitrepo.SourceUse{{Source: object}}}}
			var out, diagnostic bytes.Buffer
			if code := sourceManageCommand(context.Background(), c, []string{"list", "--json"}, strings.NewReader(""), &out, &diagnostic); code != 0 {
				t.Fatal(code, diagnostic.String())
			}
			if language == "en" {
				englishJSON = out.String()
			} else if out.String() != englishJSON {
				t.Fatal("locale changed machine output")
			}
			out.Reset()
			c.calls = nil
			c.err = errors.New("backend original detail: recovery_required")
			if code := sourceManageCommand(context.Background(), c, []string{"delete", "--yes", "source"}, unexpectedConfirmationRead{t}, &out, &diagnostic); code != 1 {
				t.Fatal(code)
			}
			if !strings.Contains(diagnostic.String(), c.err.Error()) || strings.Contains(out.String(), cliMessage("source.deleted")) {
				t.Fatal("raw failure lost or failed operation reported successful")
			}
			if language == "en" {
				englishRequests = c.calls
			} else if !reflect.DeepEqual(englishRequests, c.calls) {
				t.Fatal("locale changed reviewed deletion identity")
			}
		})
	}
}

func TestSourceDeletionDoesNotDispatchWhenWarningOrPromptFails(t *testing.T) {
	for _, yes := range []bool{false, true} {
		for _, remaining := range []int{0, 1} {
			if yes && remaining == 1 {
				continue
			}
			c := &sourceClientFake{all: controlapi.RepositoryManageResponse{Sources: []gitrepo.SourceUse{{Source: gitrepo.Object{Kind: "repo", ID: "source", Owner: strings.Repeat("a", 32), State: "ready"}}}}}
			args := []string{"delete", "source"}
			if yes {
				args = []string{"delete", "--yes", "source"}
			}
			code := sourceManageCommand(context.Background(), c, args, unexpectedConfirmationRead{t}, io.Discard, &confirmationWriter{remaining})
			if code != 1 || len(c.calls) != 1 || c.calls[0].Operation != "list" {
				t.Fatalf("failed display dispatched delete: code=%d calls=%+v", code, c.calls)
			}
		}
	}
}

func TestImageDeletionLocalePreservesReviewedValuesAndCompletion(t *testing.T) {
	id := "sha256:" + strings.Repeat("a", 64)
	target := oci.ImageTarget{Environment: "dev", Instance: "env-" + strings.Repeat("b", 32), Store: core.PersistentResourceRef{ID: "oci:dev", Owner: strings.Repeat("c", 32)}, Runtime: "nerdctl"}
	for _, language := range []string{"en", "ja"} {
		t.Run(language, func(t *testing.T) {
			t.Setenv("HACO_UI_LANGUAGE", language)
			c := &imageReviewClient{result: oci.ManagedImageList{Target: target, Images: []oci.ManagedImage{{ID: id, Tags: []string{"app:dev"}}}, IndependentSnapshots: []string{"saved"}}}
			var out, diagnostic bytes.Buffer
			code := ociImageManageCommand(context.Background(), c, []string{"delete", "--yes", "dev", "app:dev"}, unexpectedConfirmationRead{t}, &out, &diagnostic)
			if code != 0 || len(c.requests) != 2 || c.requests[1].Target != target || c.requests[1].ID != id {
				t.Fatal(code, c.requests, diagnostic.String())
			}
			for _, literal := range []string{id, target.Instance, target.Store.Owner, "app:dev", "saved"} {
				if !strings.Contains(out.String(), literal) {
					t.Fatalf("review/completion lost %q: %s", literal, out.String())
				}
			}
			if strings.Contains(out.String(), "%!") || strings.Contains(diagnostic.String(), "%!") {
				t.Fatal("message placeholders were formatted more than once")
			}
			if language == "ja" && !strings.Contains(out.String(), "OCIイメージを削除しました: "+id) {
				t.Fatal("Japanese completion missing", out.String())
			}
		})
	}
}

func TestUnusedImageDeletionStopsAtFailedImpactOrCompletionDisplay(t *testing.T) {
	target := oci.ImageTarget{Environment: "dev", Instance: "env-" + strings.Repeat("b", 32), Store: core.PersistentResourceRef{ID: "oci:dev", Owner: strings.Repeat("c", 32)}, Runtime: "nerdctl"}
	for _, phase := range []string{"unused-warning", "delete-warning", "completion"} {
		t.Run(phase, func(t *testing.T) {
			c := &unusedReviewClient{result: oci.ManagedImageList{Target: target, Images: []oci.ManagedImage{{ID: "sha256:" + strings.Repeat("a", 64)}, {ID: "sha256:" + strings.Repeat("d", 64)}}}}
			var out, diagnostic io.Writer = io.Discard, &confirmationWriter{0}
			wantDeletes := 0
			if phase == "delete-warning" {
				diagnostic = &confirmationWriter{1}
			}
			if phase == "completion" {
				// Fail only after the first real client mutation, independent of
				// the tabwriter's number of writes while rendering the review.
				out, diagnostic, wantDeletes = completionFailureWriter{c}, io.Discard, 1
			}
			code := ociImageManageCommand(context.Background(), c, []string{"delete", "--unused", "--yes", "dev"}, unexpectedConfirmationRead{t}, out, diagnostic)
			if code != 1 || len(c.deleted) != wantDeletes {
				t.Fatalf("code=%d deletes=%d want=%d", code, len(c.deleted), wantDeletes)
			}
		})
	}
}
