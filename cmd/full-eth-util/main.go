package main

import (
	"context"
	"fmt"
	"log"
	"net"
	"os"
	"os/exec"
	"strings"
	"text/template"
	"time"
)

// Флоу утилиты:
// 1.   передать eth порты и подготовить пары для тестирования
// 2.   Для каждой пары подготовить окружение из заранее заготовленных скриптов
//   2.1. для серверного порта
//   2.2. для клиентского порта
// 3.   запустить горутину сервера на серверном порту
// 4.   запустить командной утилиту клиента на клиентском порту
// 5.   серверная горутина считает полученные пакеты и выдаёт результаты в канал
// 6.   сравнить результаты

type Ethernet struct {
	Name string     `yaml:"name"`
	Ip string `yaml:"ip"`
	Port string `yaml:"port"`
}

type args struct {
		NS  string
		IP  string
		Eth string
	}

var ports = []string{
	"enp1s0",
	"enp2s0",
}

var eths = []Ethernet{{Name: "enp1s0", Ip: "200.200.200.201", Port: "8764"}, {Name: "enp2s0", Ip: "200.200.200.202", Port: "8764"}}

const nameNetNamespace = "testns"

func loadTemplates() (map[string]*template.Template, error) {
	result := map[string]*template.Template{}
	patterns := map[string]string{
		"preTest":     "ip netns add {{.NS}}",
		"postTest":    "ip netns delete {{.NS}}",
		"preEachEth1":  "ip link set dev {{.Eth}} netns {{.NS}}",
		"preEachEth2": "ip netns exec {{.NS}} ip addr add {{.IP}}/24 dev {{.Eth}}",
		"preEachEth3": "ip netns exec {{.NS}} ip link set dev {{.Eth}} up",
		"preServerEth1": "ip addr add {{.IP}}/24 dev {{.Eth}}",
		"preServerEth2":  "ip link set dev {{.Eth}} up", 
		"postEachEth": "ip netns exec {{.NS}} ip link set dev {{.Eth}} netns 1",
		"postServerEth": "ip addr del {{.IP}}/24 dev {{.Eth}}",
	}

	for name, pattern := range patterns {
		tmpl, err := template.New(name).Parse(pattern)
		if err != nil {
			return nil, err
		}
		result[name] = tmpl
	}

	return result, nil
}

func splitCmd(cmd string) (string, []string) {
	cmds := strings.Split(cmd, " ")
	return cmds[0], cmds[1:]
}

func runCmd(cmd string) error {
	cmdC, cmdA := splitCmd(cmd)
	out, err := exec.Command(cmdC, cmdA...).CombinedOutput()
	if len(out) > 0 {
		fmt.Printf("> %s\n%s", cmd, string(out))
	}
	if err != nil {
		return fmt.Errorf("ошибка выполнения команды %q: %w", cmd, err)
	}
	return nil
}

func setRPFilter(iface, val string) error {
	path := fmt.Sprintf("/proc/sys/net/ipv4/conf/%s/rp_filter", iface)
	return os.WriteFile(path, []byte(val), 0)
}


func main() {
	result := make(chan int)
	// подготовка шаблонов команд
	tmpls, err := loadTemplates()
	if err != nil {
		// return PortsInfo{}, err
		log.Fatalf("Не удалось загрузить шаблоны %v", err)
	}
	fmt.Println("Шаблоны команд подготовлены")

	// 1.   передать eth порты и подготовить пары для тестирования
	var pairs = map[Ethernet]Ethernet{
		eths[0]: eths[1],
		// eths[1]: eths[0], 
	}
	fmt.Println("Пары eth портов подготовлены лоя тестирования")

	var cmd strings.Builder

	// выполняю действия перед тестами сетевых интерфейсов
	err = tmpls["preTest"].Execute(&cmd, args{NS: nameNetNamespace})
	if err != nil {
		fmt.Println(fmt.Errorf("ошибка создания шаблона пре-теста %v", err))
		return
	}
	if err = runCmd(cmd.String()); err != nil {
		fmt.Println(fmt.Errorf("ошибка выполнения пре-теста %v %s", err, cmd.String()))
		return
	}

	defer func ()  {
		// выполняю действия после тестов сетевых интерфейсов
		var lastCmd strings.Builder
		err = tmpls["postTest"].Execute(&lastCmd, args{NS: nameNetNamespace})
		if err != nil {
			fmt.Println(fmt.Errorf("ошибка создания шаблона пост-тестовых действий %v", err))
			return // PortsInfo{}, 
		}
		if err = runCmd(lastCmd.String()); err != nil {
			fmt.Println(fmt.Errorf("ошибка выполнения пост-тестовых действий %v", err))
		}
	}()

	// rp_filter в строгом режиме дропает пакеты с src из другого netns.
	// Для стенда с двумя портами в разных namespace'ах отключаем проверку.
	for _, iface := range []string{"all", "default", ports[0], ports[1]} {
		if err := setRPFilter(iface, "0"); err != nil {
			fmt.Printf("не удалось выключить rp_filter для %s: %v\n", iface, err)
		}
	}

	for client, server := range pairs {
		// 2.   Для каждой пары подготовить окружение из заранее заготовленных скриптов
		//   2.1. для серверного порта
		//   2.2. для клиентского порта
		func() {
			// действия для клиентского порта
			for _, actions := range []string{"preEachEth1","preEachEth2","preEachEth3"} {
				cmd.Reset()
				var text string
				switch actions {
				case "preEachEth1":
					text = fmt.Sprintf("Перевожу порт %s с адресом %s в пространство %s", client.Name, client.Ip, nameNetNamespace)
				case "preEachEth2":
					text = fmt.Sprintf("Ставлю адрес %s для порта %s в пространстве %s", client.Ip, client.Name, nameNetNamespace)
				case "preEachEth3":
					text = fmt.Sprintf("Делаю статус UP для порта %s в пространстве %s", client.Name, nameNetNamespace)
				}
				fmt.Println(text)

				if err = tmpls[actions].Execute(&cmd, args{NS: nameNetNamespace, Eth: client.Name, IP: client.Ip}); err != nil {
					fmt.Println(fmt.Sprintf("ошибка подготовки шаблона команды для порта-клиента %s %v", client.Name, err))
					return 
				}
				if err = runCmd(cmd.String()); err != nil {
					fmt.Println(fmt.Errorf("ошибка выполения пре-eth действий для порта клиента %s %v команда: %s", client.Name, err, cmd.String()))
					return 
				}
			}

			defer func() {
				var cmdL strings.Builder
				if err = tmpls["postEachEth"].Execute(&cmdL, args{NS: nameNetNamespace, Eth: client.Name}); err != nil {
					fmt.Println(err.Error())
					return
				}
				if err = runCmd(cmdL.String()); err != nil {
					fmt.Println(err.Error())
				}
			}()

			fmt.Println(fmt.Sprintf("Запуск сервера слушателя на порту %s с адресом %s", server.Name, server.Ip))
			
			func ()  {
				for _, action := range []string{"preServerEth1", "preServerEth2"} {
					cmd.Reset()
					if err = tmpls[action].Execute(&cmd, args{Eth: server.Name, IP: server.Ip}); err != nil {
						fmt.Println(err.Error())
						return 
					}
					if err = runCmd(cmd.String()); err != nil {
						fmt.Println(err.Error())
						return
					}
				}
			}()

			defer func() {
				var cmdLocal strings.Builder
				if err = tmpls["postServerEth"].Execute(&cmdLocal, args{Eth: server.Name, IP: server.Ip}); err != nil {
					fmt.Println(err.Error())
				}
				if err = runCmd(cmdLocal.String()); err != nil {
					fmt.Println(err.Error())
				}
			}()

			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			
			// 3.   запустить горутину сервера на серверном порту
			fmt.Println("Запуск сервера")
			go listen(ctx, result, server.Ip, server.Port)
			time.Sleep(200 * time.Millisecond) // даём слушателю забиндить сокет до старта клиента

			// 4.   запустить командной утилиту клиента на клиентском порту
			fmt.Println("Запуск клиента")
			cmd.Reset()
			// TODO: законфижить путь до бинарника отрпавителя
			if err = runCmd(fmt.Sprintf("ip netns exec %s ./eth-util --mode=client --ip=%s", nameNetNamespace, server.Ip)); err != nil {
				fmt.Println(fmt.Errorf("не удалось запустить клиента для порта %s %v", client.Name, err))
				return 
			}

			// 5.   серверная горутина считает полученные пакеты и выдаёт результаты в канал
			actual := <- result
			if actual == expectedCount {
				fmt.Println("Тест пройден")
			} else {
				fmt.Println("Тест провален")
				fmt.Println("Получено пакетов: ", actual)
			}

		}()
		

	}

	

}

// func main() {
// 	var mode, ip string 
// 	flag.StringVar(&mode, "mode", "server", "workmode for util (server,client)")
// 	flag.StringVar(&ip, "ip", "localhost", "target ip")
// 	flag.Parse()

// 	port := "8764"

// 	switch mode {
// 	case "server":
// 		fmt.Println("server")
// 		result := make(chan int)
// 		ctx, cancel := context.WithTimeout(context.Background(), 30 * time.Second)
// 		defer cancel()
// 		go listen(ctx, result, ip, port)
// 		actual := <- result
// 		if actual == expectedCount {
// 			fmt.Println("Тест пройден")
// 		} else {
// 			fmt.Println("Тест провален")
// 			fmt.Println("Получено пакетов: ", actual)
// 		}
// 	case "client":
// 		fmt.Println("client")
// 		send(ip, port)
// 	default:
// 		log.Fatalf("Недопустимый режим работы %s. Допустимы режим server,client", mode)
// 	}
// }

const (
	expectedMsg = "expected message"
	expectedCount = 1_000
)

func listen(ctx context.Context, result chan int, ip, port string) {
	address := fmt.Sprintf("%s:%s", ip, port)
	conn, err := net.ListenPacket("udp", address)
	if err != nil {
		fmt.Println("listen:", err)
		result <- -1
		return
	}
	defer conn.Close()
	if err = conn.SetReadDeadline(time.Now().Add(5 * time.Second)); err != nil {
		fmt.Println("Не удалось установить дедлайн на чтение")
		result <- -1
		return
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
		log.Println(err)
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