// Package sidecar launches and talks to the Node login bridge that performs
// 汽水音乐 (Qishui) and 酷狗音乐 (KuGou) QR sign-ins. Those two services'
// signing lives in npm packages that Go cannot reproduce, so a small Node
// process runs them and serves a tiny local HTTP API the app drives.
package sidecar

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// Account is a signed-in user as the bridge reports it.
type Account struct {
	ID     string `json:"id"`
	Name   string `json:"nickname"`
	Avatar string `json:"avatar"`
	IsVip  bool   `json:"isVip"`
}

// Client drives one sidecar process.
type Client struct {
	port   int
	http   *http.Client
	cmd    *exec.Cmd
	stdin  io.WriteCloser
	stdout io.ReadCloser
}

// startResp is the bridge's ready payload written to the port file / stdout.
type startResp struct {
	Ready bool `json:"ready"`
	Port  int  `json:"port"`
}

// LocateNode finds a node binary to run the bridge with.
func LocateNode() (string, error) {
	if p, err := exec.LookPath("node"); err == nil {
		return p, nil
	}
	for _, p := range []string{
		"/usr/local/bin/node",
		"/opt/homebrew/bin/node",
		"/usr/bin/node",
		filepath.Join(os.Getenv("HOME"), "homebrew", "bin", "node"),
	} {
		if _, err := os.Stat(p); err == nil {
			return p, nil
		}
	}
	return "", fmt.Errorf("未找到 node：请安装 Node.js 或使用 ECHO_NODE_BIN 指定")
}

// LocateDir finds the sidecar/ directory that holds index.cjs.
func LocateDir() string {
	if d := os.Getenv("ECHO_SIDECAR_DIR"); d != "" {
		return d
	}
	if exe, err := os.Executable(); err == nil {
		exeDir := filepath.Dir(exe)
		// 打包后的 .app：Contents/MacOS/sidecar，或 Contents/Resources/sidecar。
		for _, rel := range []string{"sidecar", "..", filepath.Join("..", "Resources", "sidecar")} {
			candidate := filepath.Join(exeDir, rel)
			if _, err := os.Stat(filepath.Join(candidate, "index.cjs")); err == nil {
				return candidate
			}
		}
	}
	// 开发时从仓库根目录运行：沿当前目录向上找 sidecar/index.cjs。
	dir, _ := os.Getwd()
	for i := 0; i < 8; i++ {
		candidate := filepath.Join(dir, "sidecar")
		if _, err := os.Stat(filepath.Join(candidate, "index.cjs")); err == nil {
			return candidate
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return "sidecar"
}

// Start spawns the bridge and waits for it to report its port.
func Start() (*Client, error) {
	node, err := LocateNode()
	if err != nil {
		return nil, err
	}
	dir := LocateDir()
	script := filepath.Join(dir, "index.cjs")
	if _, err := os.Stat(script); err != nil {
		return nil, fmt.Errorf("未找到登录桥接脚本 %s（用 ECHO_SIDECAR_DIR 指定）", script)
	}

	portFile, err := os.CreateTemp("", "echo-sidecar-*.port")
	if err != nil {
		return nil, err
	}
	portFile.Close()
	_ = os.Remove(portFile.Name()) // 让子进程创建它

	cmd := exec.Command(node, script)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "ECHO_SIDECAR_PORTFILE="+portFile.Name())
	// 子进程的日志丢弃，避免干扰。
	cmd.Stdout = io.Discard
	cmd.Stderr = io.Discard
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("启动登录桥接失败：%w", err)
	}

	port, err := waitPort(portFile.Name(), cmd)
	if err != nil {
		_ = cmd.Process.Kill()
		return nil, err
	}

	return &Client{
		port: port,
		http: &http.Client{Timeout: 30 * time.Second},
		cmd:  cmd,
	}, nil
}

func waitPort(file string, cmd *exec.Cmd) (int, error) {
	deadline := time.Now().Add(15 * time.Second)
	for time.Now().Before(deadline) {
		if cmd.ProcessState != nil && cmd.ProcessState.Exited() {
			return 0, fmt.Errorf("登录桥接进程已退出")
		}
		b, err := os.ReadFile(file)
		if err == nil && len(b) > 0 {
			var r startResp
			if json.Unmarshal(b, &r) == nil && r.Port > 0 {
				return r.Port, nil
			}
		}
		time.Sleep(120 * time.Millisecond)
	}
	return 0, fmt.Errorf("登录桥接未在预期时间内就绪")
}

// Stop terminates the bridge.
func (c *Client) Stop() {
	if c.cmd != nil && c.cmd.Process != nil {
		_ = c.cmd.Process.Kill()
	}
}

func (c *Client) call(provider, action string, req, resp any) error {
	return c.callWithin(provider, action, req, resp, 0)
}

// callWithin is call, waiting up to wait for the answer when it is over
// zero. Resolving a track's play address makes the bridge fetch the whole
// song first (and transcode it when the container is one the player cannot
// read), which the default 30s timeout cuts short on a slow line.
func (c *Client) callWithin(provider, action string, req, resp any, wait time.Duration) error {
	var body []byte
	if req != nil {
		b, err := json.Marshal(req)
		if err != nil {
			return err
		}
		body = b
	}
	u := fmt.Sprintf("http://127.0.0.1:%d/v1/%s/%s", c.port, provider, action)
	httpReq, err := http.NewRequest(http.MethodPost, u, strings.NewReader(string(body)))
	if err != nil {
		return err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	client := c.http
	if wait > 0 {
		client = &http.Client{Timeout: wait}
	}
	resp2, err := client.Do(httpReq)
	if err != nil {
		return err
	}
	defer resp2.Body.Close()
	out, _ := io.ReadAll(resp2.Body)
	// 整个响应即业务载荷；先试着解析 error 字段。
	var probe struct {
		Error string `json:"error"`
	}
	if json.Unmarshal(out, &probe) == nil && probe.Error != "" {
		return fmt.Errorf("%s", probe.Error)
	}
	if resp != nil {
		if err := json.Unmarshal(out, resp); err != nil {
			return fmt.Errorf("解析桥接响应失败：%w（%s）", err, string(out))
		}
	}
	return nil
}

// StartQR begins a QR login and returns the key plus the QR image (PNG, base64).
func (c *Client) StartQR(provider string) (key, image string, err error) {
	var r struct {
		Key   string `json:"key"`
		Image string `json:"image"`
		Text  string `json:"text"`
		Error string `json:"error"`
	}
	if err := c.call(provider, "start", nil, &r); err != nil {
		return "", "", err
	}
	if r.Error != "" {
		return "", "", fmt.Errorf("%s", r.Error)
	}
	return r.Key, r.Image, nil
}

// CheckQR polls a QR login; state is waiting|scanned|confirmed|expired.
func (c *Client) CheckQR(provider, key string) (state string, err error) {
	var r struct {
		State string `json:"state"`
		Error string `json:"error"`
	}
	if err := c.call(provider, "check", map[string]string{"key": key}, &r); err != nil {
		return "", err
	}
	if r.Error != "" {
		return "", fmt.Errorf("%s", r.Error)
	}
	return r.State, nil
}

// Session returns the credentials the bridge holds for provider, to persist.
func (c *Client) Session(provider string) (json.RawMessage, error) {
	var r json.RawMessage
	if err := c.call(provider, "session", nil, &r); err != nil {
		return nil, err
	}
	return r, nil
}

// Restore puts a persisted session back into the bridge.
func (c *Client) Restore(provider string, creds json.RawMessage) error {
	return c.call(provider, "restore", creds, nil)
}

// Status reports who is signed in, or nil when nobody is.
func (c *Client) Status(provider string) (*Account, error) {
	var r *Account
	if err := c.call(provider, "status", nil, &r); err != nil {
		return nil, err
	}
	return r, nil
}

// Logout clears the bridge's in-memory credentials for provider.
func (c *Client) Logout(provider string) error {
	return c.call(provider, "logout", nil, nil)
}

// Shared is the process-wide sidecar client. The bridge serves every
// provider from one Node process, so all callers share it. The first call
// starts the process; later calls reuse it until StopShared.
var (
	sharedMu  sync.Mutex
	shared    *Client
	sharedErr error
)

// Shared returns the shared sidecar client, starting it on first use.
func Shared() (*Client, error) {
	sharedMu.Lock()
	defer sharedMu.Unlock()
	if shared != nil || sharedErr != nil {
		return shared, sharedErr
	}
	shared, sharedErr = Start()
	return shared, sharedErr
}

// StopShared tears the shared sidecar down; the next Shared() starts fresh.
func StopShared() {
	sharedMu.Lock()
	defer sharedMu.Unlock()
	if shared != nil {
		shared.Stop()
		shared = nil
	}
	sharedErr = nil
}
