package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestDotSingleFile(t *testing.T) {
	canonical, err := os.ReadFile("../../dot/client.py")
	if err != nil {
		t.Fatal(err)
	}
	if string(canonical) != dotClient {
		t.Fatal("embedded client differs from dot/client.py")
	}
	python, err := exec.LookPath("python3")
	if err != nil {
		t.Skip("python3 unavailable")
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "dot-reader.py")
	key := "test-key-with-quotes-\"-'\\-and-unicode-한글"
	if err := os.WriteFile(path, []byte(dotScript("https://example.com", key)), 0600); err != nil {
		t.Fatal(err)
	}
	// Mock transport verifies default operation and embedded credentials without network.
	script := `import runpy, sys, io, json
m = runpy.run_path(sys.argv[1], run_name='test_client')
assert m['EMBEDDED_CONFIG']['readKey'] == sys.argv[2]
expected_key = sys.argv[2]
class Transport:
 def open(self, req, timeout):
  assert req.full_url == 'https://example.com/v1/read/relay_status'
  assert req.get_header('Authorization') == 'Bearer '+expected_key
  return io.BytesIO(b'{"ok":true}')
m['urllib'].request.build_opener = lambda *a: Transport()
sys.argv = [sys.argv[1]]
m['main']()
`
	if output, err := exec.Command(python, "-c", script, path, key).CombinedOutput(); err != nil {
		t.Fatalf("generated client failed: %v %s", err, output)
	}
}
