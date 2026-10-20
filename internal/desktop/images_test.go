package desktop

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestWebPDimensionHeadersRejectUnknownAndBoundSizes(t *testing.T) {
	data := make([]byte, 30)
	copy(data, "RIFF")
	copy(data[8:], "WEBP")
	copy(data[12:], "VP8X")
	data[24] = 99
	data[27] = 49
	if w, h := webpSize(data); w != 100 || h != 50 {
		t.Fatal(w, h)
	}
	copy(data[12:], "VP8L")
	data[20] = 0x2f
	binary.LittleEndian.PutUint32(data[21:25], 99|(49<<14))
	if w, h := webpSize(data); w != 100 || h != 50 {
		t.Fatal(w, h)
	}
	copy(data[12:], "VP8 ")
	copy(data[23:], []byte{0x9d, 0x01, 0x2a})
	binary.LittleEndian.PutUint16(data[26:28], 100)
	binary.LittleEndian.PutUint16(data[28:30], 50)
	if w, h := webpSize(data); w != 100 || h != 50 {
		t.Fatal(w, h)
	}
	if w, h := webpSize(data[:20]); w != 0 || h != 0 {
		t.Fatal("truncated header accepted")
	}
	copy(data[12:], "JUNK")
	if w, h := webpSize(data); w != 0 || h != 0 {
		t.Fatal("unknown header accepted")
	}
}

func TestMessageImagesAreReferencedRootedAndRasterOnly(t *testing.T) {
	b, _, _ := controlFixture(t)
	defer b.Close()
	row, err := b.History.Get(context.Background(), "thread")
	if err != nil {
		t.Fatal(err)
	}
	var pngBytes bytes.Buffer
	png.Encode(&pngBytes, image.NewRGBA(image.Rect(0, 0, 3, 2)))
	allowed := filepath.Join(row.CWD, "returned image.png")
	os.WriteFile(allowed, pngBytes.Bytes(), 0600)
	outside := filepath.Join(t.TempDir(), "private.png")
	os.WriteFile(outside, pngBytes.Bytes(), 0600)
	svg := filepath.Join(row.CWD, "script.svg")
	os.WriteFile(svg, []byte(`<svg xmlns="http://www.w3.org/2000/svg"><script>alert(1)</script></svg>`), 0600)
	items := []any{
		map[string]any{"id": "local", "type": "agentMessage", "text": "![result](<" + filepath.ToSlash(allowed) + ">)"},
		map[string]any{"id": "outside", "type": "agentMessage", "text": "![blocked](<" + filepath.ToSlash(outside) + ">)"},
		map[string]any{"id": "svg", "type": "agentMessage", "text": "![blocked](<" + filepath.ToSlash(svg) + ">)"},
		map[string]any{"id": "generated", "type": "imageGenerationCall", "result": base64.StdEncoding.EncodeToString(pngBytes.Bytes())},
		map[string]any{"id": "remote", "type": "agentMessage", "text": "![tracking](https://evil.example/track.png)"},
	}
	raw := map[string]any{"turns": []any{map[string]any{"turnId": "turn", "items": items}}}
	b.sessions["thread"] = &State{ID: "thread", Snapshot: mustJSON(raw)}
	for index, entry := range items {
		item := entry.(map[string]any)
		refs := imageReferences(item)
		if index == 4 {
			if len(refs) != 0 {
				t.Fatal("remote image allowed")
			}
			continue
		}
		if len(refs) != 1 {
			t.Fatal("image reference lost", index, refs)
		}
		data, mime, err := b.Image(context.Background(), "thread", "turn", item["id"].(string), refs[0].ID)
		if index == 1 || index == 2 {
			if err == nil {
				t.Fatal("outside-root or active content image read")
			}
			continue
		}
		if err != nil || mime != "image/png" || !bytes.Equal(data, pngBytes.Bytes()) {
			t.Fatal("valid image unavailable", index, err)
		}
	}
	if _, _, err = b.Image(context.Background(), "thread", "turn", "local", strings.Repeat("f", 64)); err == nil {
		t.Fatal("unreferenced image accepted")
	}
	projected := PhoneState(*b.sessions["thread"])
	if strings.Contains(string(projected.Snapshot), base64.StdEncoding.EncodeToString(pngBytes.Bytes())) {
		t.Fatal("inline image bytes leaked into websocket")
	}
	var out map[string]any
	json.Unmarshal(projected.Snapshot, &out)
	generated := orderedTurns(out)[0].(map[string]any)["items"].([]any)[3].(map[string]any)
	if len(generated["images"].([]any)) != 1 {
		t.Fatal("image-only native result omitted")
	}
	if err = os.Symlink(outside, filepath.Join(row.CWD, "escape.png")); err == nil {
		link := map[string]any{"id": "link", "type": "agentMessage", "text": "![escape](escape.png)"}
		raw["turns"].([]any)[0].(map[string]any)["items"] = []any{link}
		b.sessions["thread"].Snapshot = mustJSON(raw)
		if _, _, err = b.Image(context.Background(), "thread", "turn", "link", imageReferences(link)[0].ID); err == nil {
			t.Fatal("symlink escaped image root")
		}
	}
}

func TestNativeImageGenerationSavedPathProjectsWithoutDeferredBody(t *testing.T) {
	b, _, _ := controlFixture(t)
	defer b.Close()
	row, err := b.History.Get(context.Background(), "thread")
	if err != nil {
		t.Fatal(err)
	}
	var data bytes.Buffer
	png.Encode(&data, image.NewRGBA(image.Rect(0, 0, 3, 2)))
	path := filepath.Join(row.CWD, "native-generated.png")
	if err = os.WriteFile(path, data.Bytes(), 0600); err != nil {
		t.Fatal(err)
	}
	native := map[string]any{"id": "generated-native", "type": "imageGeneration", "savedPath": path, "src": "data:image/png;base64," + base64.StdEncoding.EncodeToString(data.Bytes()), "result": "", "status": "completed", "revisedPrompt": "hidden prompt"}
	refs := imageReferences(native)
	if len(refs) != 1 || refs[0].Source != path {
		t.Fatal("savedPath lost or image duplicated", len(refs))
	}
	state := State{ID: "thread", Snapshot: mustJSON(map[string]any{"turns": []any{map[string]any{"turnId": "turn", "items": []any{native}}}})}
	b.sessions["thread"] = &state
	for _, lazy := range []bool{false, true} {
		out := projectState(state, lazy)
		var raw map[string]any
		json.Unmarshal(out.Snapshot, &raw)
		item := orderedTurns(raw)[0].(map[string]any)["items"].([]any)[0].(map[string]any)
		if item["type"] != "imageGenerationCall" || item["deferred"] == true || len(item["images"].([]any)) != 1 {
			t.Fatal("native image hidden by projection", item)
		}
		if strings.Contains(string(out.Snapshot), "hidden prompt") || strings.Contains(string(out.Snapshot), "data:image") {
			t.Fatal("image metadata leaked")
		}
	}
	got, mime, err := b.Image(context.Background(), "thread", "turn", "generated-native", refs[0].ID)
	if err != nil || mime != "image/png" || !bytes.Equal(got, data.Bytes()) {
		t.Fatal("native saved image unavailable", err)
	}
}

func TestHistoryImagegenToolOutputPreservesOnlyImageContent(t *testing.T) {
	b, _, _ := controlFixture(t)
	defer b.Close()
	row, err := b.History.Get(context.Background(), "thread")
	if err != nil {
		t.Fatal(err)
	}
	var data bytes.Buffer
	png.Encode(&data, image.NewRGBA(image.Rect(0, 0, 2, 2)))
	uri := "data:image/png;base64," + base64.StdEncoding.EncodeToString(data.Bytes())
	rows := []any{
		map[string]any{"type": "event_msg", "payload": map[string]any{"type": "task_started", "turn_id": "real-turn"}},
		map[string]any{"type": "response_item", "payload": map[string]any{"type": "message", "role": "user", "content": []any{map[string]any{"type": "input_text", "text": "draw a box"}}}},
		map[string]any{"type": "response_item", "payload": map[string]any{"type": "function_call", "name": "imagegen", "call_id": "image-call", "arguments": "hidden generation arguments"}},
		map[string]any{"type": "response_item", "payload": map[string]any{"type": "function_call_output", "call_id": "image-call", "output": []any{map[string]any{"type": "input_image", "image_url": uri}, map[string]any{"type": "input_text", "text": "hidden image tool instructions"}}}},
		map[string]any{"type": "response_item", "payload": map[string]any{"type": "function_call_output", "call_id": "unrelated-call", "output": []any{map[string]any{"type": "input_image", "image_url": uri}}}},
		map[string]any{"type": "response_item", "payload": map[string]any{"type": "message", "role": "assistant", "content": []any{map[string]any{"type": "output_text", "text": "done"}}}},
	}
	row.RolloutPath = filepath.Join(b.config.CodexHome, "image-tool.jsonl")
	var log bytes.Buffer
	for _, line := range rows {
		encoded, _ := json.Marshal(line)
		log.Write(encoded)
		log.WriteByte('\n')
	}
	if err = os.WriteFile(row.RolloutPath, log.Bytes(), 0600); err != nil {
		t.Fatal(err)
	}
	raw, err := b.readHistoryValue(context.Background(), row, true)
	if err != nil {
		t.Fatal(err)
	}
	projected := projectState(State{document: &nativeDocument{value: raw}}, true)
	var out map[string]any
	json.Unmarshal(projected.Snapshot, &out)
	count := 0
	for _, turn := range orderedTurns(out) {
		for _, entry := range orderedItems(turn.(map[string]any)["items"]) {
			item := entry.(map[string]any)
			if item["type"] == "imageGenerationCall" {
				count++
				if item["deferred"] == true || len(item["images"].([]any)) != 1 {
					t.Fatal("historical image hidden")
				}
			}
		}
	}
	if count != 1 {
		t.Fatal("paired imagegen output omitted or unrelated output exposed", count)
	}
	if strings.Contains(string(projected.Snapshot), "hidden") || strings.Contains(string(projected.Snapshot), "base64") {
		t.Fatal("raw tool metadata forwarded")
	}
}

func TestWindowsImagePathSpellingsKeepRootedContainment(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("Win32 extended path equivalence")
	}
	b, _, _ := controlFixture(t)
	defer b.Close()
	row, err := b.History.Get(context.Background(), "thread")
	if err != nil {
		t.Fatal(err)
	}
	var data bytes.Buffer
	if err = png.Encode(&data, image.NewRGBA(image.Rect(0, 0, 3, 2))); err != nil {
		t.Fatal(err)
	}
	inside := filepath.Join(row.CWD, "returned image.png")
	outside := filepath.Join(t.TempDir(), "private.png")
	sibling := row.CWD + "-sibling"
	if err = os.Mkdir(sibling, 0700); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Remove(filepath.Join(sibling, "private.png")); os.Remove(sibling) })
	siblingFile := filepath.Join(sibling, "private.png")
	for _, p := range []string{inside, outside, siblingFile} {
		if err = os.WriteFile(p, data.Bytes(), 0600); err != nil {
			t.Fatal(err)
		}
	}
	db, err := sql.Open("sqlite", filepath.Join(b.config.CodexHome, "state_5.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	for _, root := range []string{row.CWD, `\\?\` + row.CWD} {
		if _, err = db.Exec("UPDATE threads SET cwd=? WHERE id='thread'", root); err != nil {
			t.Fatal(err)
		}
		for i, path := range []string{filepath.ToSlash(inside), `\\?\` + inside, "returned image.png", outside, `\\?\` + outside, siblingFile, filepath.Join("..", filepath.Base(sibling), "private.png")} {
			item := map[string]any{"id": "image", "type": "agentMessage", "text": "![result](<" + path + ">)"}
			b.sessions["thread"] = &State{ID: "thread", Snapshot: mustJSON(map[string]any{"turns": []any{map[string]any{"turnId": "turn", "items": []any{item}}}})}
			refs := imageReferences(item)
			if len(refs) != 1 {
				t.Fatal("reference lost")
			}
			got, mime, err := b.Image(context.Background(), "thread", "turn", "image", refs[0].ID)
			if i < 3 {
				if err != nil || mime != "image/png" || !bytes.Equal(got, data.Bytes()) {
					t.Fatalf("equivalent in-root path rejected: cwd=%q source=%q err=%v", root, path, err)
				}
			} else if err == nil {
				t.Fatal("outside-root path accepted", root, path)
			}
		}
	}
}
