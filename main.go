package main

import (
	"flag"
	"fmt"
	"gopkg.in/robfig/cron.v2"
	"kafka-repush/services"
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"
)

type (
	serviceConfig struct {
		isDefault        bool
		logFileName      string
		lastLineFileName string
		failPushFileName string
		cronFormat       string
		broker           []string
	}
)

var (
	//brokers = []string{"192.168.75.132:9092"}
	config serviceConfig
)

func main() {

	defaultCmd := flag.NewFlagSet("default", flag.ExitOnError)
	defLogName := defaultCmd.String("log-name", "", "Log file name")
	defLastLineName := defaultCmd.String("last-line-name", "", "File name to store last readied line")
	defFailName := defaultCmd.String("fail-name", "", "File name to store failed push")
	defBroker := defaultCmd.String("broker", "", "Kafka broker")

	scheduleCmd := flag.NewFlagSet("schedule", flag.ExitOnError)
	slogName := scheduleCmd.String("log-name", "", "Log file name")
	sLastLineName := scheduleCmd.String("last-line-name", "", "File name to store last readied line")
	sFailName := scheduleCmd.String("fail-name", "", "File name to store failed push")
	sBroker := scheduleCmd.String("broker", "", "Kafka broker")
	cronFormat := scheduleCmd.String("cron-format", "", "Cron format")

	if len(os.Args) < 2 {
		fmt.Println("expected 'default' or 'schedule' subcommands")
		os.Exit(1)
	}
	switch os.Args[1] {
	case "default":

		if err := defaultCmd.Parse(os.Args[2:]); err != nil {
			log.Fatalln(err)
		}
		fmt.Println("Service run with default config")
		fmt.Println("  Log file's name:", *defLogName)
		fmt.Println("  Last line file's name:", *defLastLineName)
		fmt.Println("  Fail file's Name:", *defFailName)
		fmt.Println("  Broker:", *defBroker)

		config = serviceConfig{
			isDefault:        true,
			logFileName:      *defLogName,
			lastLineFileName: *defLastLineName,
			failPushFileName: *defFailName,
			broker:           strings.Split(*defBroker, " "),
		}

	case "schedule":
		if err := scheduleCmd.Parse(os.Args[2:]); err != nil {
			log.Fatalln(err)
		}

		fmt.Println("Service run with schedule config")
		fmt.Println("  Log file's name:", *slogName)
		fmt.Println("  Last line file's name:", *sLastLineName)
		fmt.Println("  Fail file's Name:", *sFailName)
		fmt.Println("  Broker:", *sBroker)
		fmt.Println("  Cron format:", *cronFormat)

		config = serviceConfig{
			isDefault:        false,
			logFileName:      *slogName,
			lastLineFileName: *sLastLineName,
			failPushFileName: *sFailName,
			broker:           strings.Split(*sBroker, " "),
			cronFormat:       *cronFormat,
		}

	default:
		fmt.Println("Expected 'default' or 'schedule' subcommands")
		os.Exit(1)
	}

	startService(config)

}
func startService(config serviceConfig) {

	fmt.Println("---------------Service Running---------------")

	producer, err := services.NewProducer(config.broker)
	if err != nil {
		log.Fatal(err)
	}
	serviceConfig := services.ServiceConfig{
		FileConfig: services.FileConfig{
			LogName:      config.logFileName,
			LastLineName: config.lastLineFileName,
			FailPushName: config.failPushFileName,
		},
		KafkaProducer: producer,
	}
	service := services.NewLogHandler(serviceConfig)

	// Run with default setting and only run one time
	if config.isDefault {
		logHandlerService(service)
		if err = service.Close(); err != nil {
			log.Fatalln(err)
		}
		return
	}

	// Run with schedule setting and run with cron config
	cronService := cron.New()
	_, err = cronService.AddFunc(config.cronFormat, func() {
		logHandlerService(service)

	})
	if err != nil {
		log.Fatalln(err)
	}

	cronService.Start()
	exit := make(chan os.Signal, 1)
	signal.Notify(exit, syscall.SIGTERM, syscall.SIGINT, os.Interrupt, os.Kill)
	for {
		select {
		case <-exit:
			fmt.Println("Exiting...")
			cronService.Stop()
			if err = service.Close(); err != nil {
				log.Fatalln(err)
			}

			return
		}
	}
}

func logHandlerService(service services.LogHandler) {
	lastLine, err := service.GetLastLine()
	if err != nil {
		log.Println("Get last line failed, err: ", err)
	}

	if err := service.GetLog(); err != nil {
		log.Fatalln("Get log failed, err: ", err)
	}

	if err := service.GetFailFile(); err != nil {
		log.Fatalln("Get fail log failed, err: ", err)
	}

	newLastLine, err := service.ReadLog(lastLine)
	if err != nil {
		log.Fatalln("Read log Failed, err: ", err)
	}

	if err = service.StoreLastLine(newLastLine); err != nil {
		log.Fatalln("Store last line failed, err: ", err)
	}

	if err = service.CloseFile(); err != nil {
		log.Fatalln("Close file failed, err: ", err)
	}
}
