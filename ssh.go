package mysqlkill

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"strconv"
	"sync"

	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"
	"golang.org/x/crypto/ssh/knownhosts"
)

// sshTunnel represents a local-to-remote SSH tunnel.
type sshTunnel struct {
	LocalHost string
	LocalPort int

	listener net.Listener
	client   *ssh.Client
	once     sync.Once
}

// startSSHTunnel opens an SSH tunnel to the target host:port.
func startSSHTunnel(ctx context.Context, cfg sshConfig, targetHost string, targetPort int) (*sshTunnel, error) {
	if targetHost == "" || targetPort == 0 {
		return nil, errors.New("db host/port required for ssh tunneling")
	}

	client, err := dialSSH(ctx, cfg)
	if err != nil {
		return nil, err
	}

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		_ = client.Close()
		return nil, fmt.Errorf("listen: %w", err)
	}

	host, portStr, err := net.SplitHostPort(listener.Addr().String())
	if err != nil {
		_ = listener.Close()
		_ = client.Close()
		return nil, fmt.Errorf("listen addr: %w", err)
	}
	port, err := strconv.Atoi(portStr)
	if err != nil {
		_ = listener.Close()
		_ = client.Close()
		return nil, fmt.Errorf("listen port: %w", err)
	}

	tunnel := &sshTunnel{
		LocalHost: host,
		LocalPort: port,
		listener:  listener,
		client:    client,
	}

	go tunnel.acceptLoop(ctx, fmt.Sprintf("%s:%d", targetHost, targetPort))

	return tunnel, nil
}

// acceptLoop accepts local connections and forwards them.
func (t *sshTunnel) acceptLoop(ctx context.Context, targetAddr string) {
	for {
		conn, err := t.listener.Accept()
		if err != nil {
			return
		}
		go t.forwardConn(ctx, conn, targetAddr)
	}
}

// forwardConn forwards a single connection through SSH.
func (t *sshTunnel) forwardConn(ctx context.Context, localConn net.Conn, targetAddr string) {
	remoteConn, err := t.client.Dial("tcp", targetAddr)
	if err != nil {
		_ = localConn.Close()
		return
	}

	go func() {
		_, _ = io.Copy(remoteConn, localConn)
		_ = remoteConn.Close()
	}()
	go func() {
		_, _ = io.Copy(localConn, remoteConn)
		_ = localConn.Close()
	}()
}

// Close closes the tunnel listener and SSH client.
func (t *sshTunnel) Close() {
	t.once.Do(func() {
		if t.listener != nil {
			_ = t.listener.Close()
		}
		if t.client != nil {
			_ = t.client.Close()
		}
	})
}

// dialSSH connects to the SSH bastion using the provided config.
func dialSSH(ctx context.Context, cfg sshConfig) (*ssh.Client, error) {
	if cfg.Host == "" {
		return nil, errors.New("ssh host required")
	}
	if cfg.User == "" {
		return nil, errors.New("ssh user required")
	}

	var auths []ssh.AuthMethod
	if cfg.KeyPath != "" {
		key, err := os.ReadFile(cfg.KeyPath)
		if err != nil {
			return nil, fmt.Errorf("read ssh key: %w", err)
		}
		signer, err := ssh.ParsePrivateKey(key)
		if err != nil {
			return nil, fmt.Errorf("parse ssh key: %w", err)
		}
		auths = append(auths, ssh.PublicKeys(signer))
	}

	var agentConn net.Conn
	if sock := os.Getenv("SSH_AUTH_SOCK"); sock != "" {
		conn, err := net.Dial("unix", sock)
		if err == nil {
			agentConn = conn
			ag := agent.NewClient(conn)
			auths = append(auths, ssh.PublicKeysCallback(ag.Signers))
		}
	}

	if len(auths) == 0 {
		return nil, errors.New("no ssh auth method available: provide SSH_KEY or SSH_AUTH_SOCK")
	}

	var hostKeyCallback ssh.HostKeyCallback
	if cfg.NoStrictHostKey {
		hostKeyCallback = ssh.InsecureIgnoreHostKey()
	} else {
		if cfg.KnownHostsPath == "" {
			return nil, errors.New("known_hosts path required for strict host key checking")
		}
		callback, err := knownhosts.New(cfg.KnownHostsPath)
		if err != nil {
			return nil, fmt.Errorf("load known_hosts: %w", err)
		}
		hostKeyCallback = callback
	}

	clientCfg := &ssh.ClientConfig{
		User:            cfg.User,
		Auth:            auths,
		HostKeyCallback: hostKeyCallback,
		Timeout:         cfg.Timeout,
	}

	addr := net.JoinHostPort(cfg.Host, strconv.Itoa(cfg.Port))
	type dialResult struct {
		client *ssh.Client
		err    error
	}
	ch := make(chan dialResult, 1)
	go func() {
		client, err := ssh.Dial("tcp", addr, clientCfg)
		ch <- dialResult{client: client, err: err}
	}()

	select {
	case <-ctx.Done():
		if agentConn != nil {
			_ = agentConn.Close()
		}
		return nil, ctx.Err()
	case res := <-ch:
		if agentConn != nil {
			_ = agentConn.Close()
		}
		if res.err != nil {
			return nil, fmt.Errorf("ssh dial: %w", res.err)
		}
		return res.client, nil
	}
}
