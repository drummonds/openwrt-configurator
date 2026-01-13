package ssh

import (
	"fmt"
	"time"

	"golang.org/x/crypto/ssh"
)

// SSHExecutor defines the interface for SSH command execution
type SSHExecutor interface {
	Execute(command string) (string, error)
	ExecuteWithError(command string) (string, error)
	Close() error
}

// Client wraps an SSH client connection
type Client struct {
	client  *ssh.Client
	session *ssh.Session
}

// Connect establishes an SSH connection to the specified host
func Connect(host, username, password string) (*Client, error) {
	config := &ssh.ClientConfig{
		User: username,
		Auth: []ssh.AuthMethod{
			ssh.Password(password),
		},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(), // In production, use proper host key verification
		Timeout:         10 * time.Second,
	}

	client, err := ssh.Dial("tcp", host+":22", config)
	if err != nil {
		return nil, fmt.Errorf("failed to dial: %w", err)
	}

	return &Client{
		client: client,
	}, nil
}

// Execute runs a command on the remote host and returns the output
func (c *Client) Execute(command string) (string, error) {
	session, err := c.client.NewSession()
	if err != nil {
		return "", fmt.Errorf("failed to create session: %w", err)
	}
	defer session.Close()

	output, err := session.CombinedOutput(command)
	if err != nil {
		return string(output), fmt.Errorf("command failed: %w", err)
	}

	return string(output), nil
}

// ExecuteWithError runs a command and returns both stdout and error separately
func (c *Client) ExecuteWithError(command string) (string, error) {
	session, err := c.client.NewSession()
	if err != nil {
		return "", fmt.Errorf("failed to create session: %w", err)
	}
	defer session.Close()

	output, err := session.CombinedOutput(command)
	return string(output), err
}

// Close closes the SSH connection
func (c *Client) Close() error {
	if c.client != nil {
		return c.client.Close()
	}
	return nil
}
