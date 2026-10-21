package app

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"time"
)

const headerTimeout = 10 * time.Second

func (c *Controller) Start() error {
	c.change.Lock()
	defer c.change.Unlock()
	c.mu.RLock()
	active := c.server != nil
	c.mu.RUnlock()
	if active {
		return fmt.Errorf("API 服务已经运行")
	}
	listener, err := net.Listen("tcp", c.Config().Address())
	if err != nil {
		c.serviceError(err)
		return err
	}
	server := &http.Server{Handler: c, ReadHeaderTimeout: headerTimeout}
	c.mu.Lock()
	c.server = server
	c.service = Service{Running: true, Address: "http://" + listener.Addr().String()}
	c.mu.Unlock()
	c.AddLog("info", "API 服务已启动："+listener.Addr().String())
	go c.serve(server, listener)
	return nil
}

func (c *Controller) serve(server *http.Server, listener net.Listener) {
	err := server.Serve(listener)
	if err == nil || errors.Is(err, http.ErrServerClosed) {
		return
	}
	c.mu.Lock()
	if c.server == server {
		c.server = nil
		c.service.Running = false
		c.service.Error = err.Error()
	}
	c.mu.Unlock()
	c.AddLog("error", err.Error())
}

func (c *Controller) Stop() error {
	c.change.Lock()
	defer c.change.Unlock()
	c.mu.RLock()
	server := c.server
	c.mu.RUnlock()
	if server == nil {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()
	err := server.Shutdown(ctx)
	if err != nil {
		err = errors.Join(err, server.Close())
	}
	c.mu.Lock()
	c.server = nil
	c.service.Running = false
	c.mu.Unlock()
	if err != nil {
		c.serviceError(err)
		return err
	}
	c.AddLog("info", "API 服务已停止")
	return nil
}

func (c *Controller) serviceError(err error) {
	c.mu.Lock()
	c.service.Error = err.Error()
	c.mu.Unlock()
	c.AddLog("error", err.Error())
}
