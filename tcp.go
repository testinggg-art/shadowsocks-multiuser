package main

import (
	"fmt"
	"io"
	"log"
	"net"
	"time"

	"github.com/NetchX/shadowsocks-multiuser/socks"
)

func tcpRemote(instance *Instance, cipher func(net.Conn) net.Conn) {
	socket, err := net.Listen("tcp", fmt.Sprintf("0.0.0.0:%d", instance.Port))
	if err != nil {
		log.Printf("Failed to listen TCP on %d: %v", instance.Port, err)
		return
	}
	defer socket.Close()

	instance.TCPStarted = true

	log.Printf("Start listening TCP on %d", instance.Port)
	for instance.Started {
		client, err := socket.Accept()
		if err != nil {
			continue
		}

		go func() {
			defer client.Close()

			client.(*net.TCPConn).SetKeepAlive(true)
			client = cipher(client)

			targetAddress, err := socks.ReadAddr(client)
			if err != nil {
				return
			}

			remoteClient, err := net.Dial("tcp", targetAddress.String())
			if err != nil {
				return
			}
			defer remoteClient.Close()

			tcpRelay(instance, client, remoteClient)
		}()
	}

	instance.TCPStarted = false
	log.Printf("Stop listening TCP on %d", instance.Port)
}

func tcpRelay(instance *Instance, left, right net.Conn) error {
	type Result struct {
		Err error
	}
	channel := make(chan Result)

	go func() {
		size, err := io.Copy(right, left)
		instance.Bandwidth.IncreaseUpload(uint64(size))
		right.SetDeadline(time.Now())
		left.SetDeadline(time.Now())
		channel <- Result{err}
	}()

	size, err := io.Copy(left, right)
	instance.Bandwidth.IncreaseDownload(uint64(size))
	right.SetDeadline(time.Now())
	left.SetDeadline(time.Now())
	result := <-channel

	if err != nil {
		err = result.Err
	}

	return err
}