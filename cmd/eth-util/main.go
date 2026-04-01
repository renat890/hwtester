package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net"
	"time"
)



func main() {
	var mode string 
	flag.StringVar(&mode, "mode", "server", "workmode for util (server,client)")
	flag.Parse()

	ip := "192.168.0.101"
	port := "8764"

	switch mode {
	case "server":
		fmt.Println("server")
		result := make(chan int)
		ctx, cancel := context.WithTimeout(context.Background(), 30 * time.Second)
		defer cancel()
		go listen(ctx, result, ip, port)
		actual := <- result
		if actual == expectedCount {
			fmt.Println("Тест пройден")
		} else {
			fmt.Println("Тест провален")
			fmt.Println("Получено пакетов: ", actual)
		}
	case "client":
		fmt.Println("client")
		send(ip, port)
	default:
		log.Fatalf("Недопустимый режим работы %s. Допустимы режим server,client", mode)
	}
}

const (
	expectedMsg = "expected message"
	expectedCount = 1_000
)

func listen(ctx context.Context, result chan int, ip, port string) {
	address := fmt.Sprintf("%s:%s", ip, port)
	conn, err := net.ListenPacket("udp", address)
	if err != nil {
		log.Fatal(err)
	}
	defer conn.Close()
	if err = conn.SetReadDeadline(time.Now().Add(5 * time.Second)); err != nil {
		log.Fatal("Не удалось установить дедлайн на чтение")
	}

	count := 0

	go func ()  {
		<- ctx.Done()
		conn.Close()
	}()

	for {
		packet := make([]byte, 1024)
		n, _, err := conn.ReadFrom(packet)
		if err != nil {
			break 
		}
		if string(packet[:n]) == expectedMsg {
			count++
		}
		if count == expectedCount {
			break
		}
}

	result <- count
}

func send(ip, port string) {
	address := fmt.Sprintf("%s:%s", ip, port)
	conn, err := net.Dial("udp", address)
	if err != nil {
		log.Fatal(err)
	}
	defer conn.Close()

	for range expectedCount {
		_, err = conn.Write([]byte(expectedMsg))
		time.Sleep(time.Microsecond)
		if err != nil {
			log.Println("не отправлен пакет ",err)
		}
	}
}