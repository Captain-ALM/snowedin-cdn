package main

import (
	"fmt"
	"github.com/joho/godotenv"
	"gopkg.in/yaml.v3"
	"log"
	"os"
	"os/signal"
	"path"
	"snow.mrmelon54.xyz/snowedin/api"
	"snow.mrmelon54.xyz/snowedin/structure"
	"snow.mrmelon54.xyz/snowedin/web"
	"sync"
	"syscall"
	"time"
)

var (
	buildVersion = "develop"
	buildDate    = ""
)

func main() {
	log.Printf("[Main] Starting up Snowedin #%s (%s)\n", buildVersion, buildDate)

	wg := &sync.WaitGroup{}
	wg.Add(1)

	cwdDir, err := os.Getwd()
	if err != nil {
		log.Println(err)
	}

	err = godotenv.Load()
	if err != nil {
		log.Fatalln("Error loading .env file")
	}

	dataDir := os.Getenv("DIR_DATA")
	if dataDir == "" {
		dataDir = path.Join(cwdDir, ".data")
	}

	check(os.MkdirAll(dataDir, 0777))

	configFile, err := os.Open(path.Join(dataDir, "config.yml"))
	if err != nil {
		log.Fatalln("Failed to open config.yml")
	}

	var configYml structure.ConfigYaml
	groupsDecoder := yaml.NewDecoder(configFile)
	err = groupsDecoder.Decode(&configYml)
	if err != nil {
		log.Fatalln("Failed to parse config.yml:", err)
	}

	webServer := web.New(configYml)
	apiServer := api.New(configYml)

	//=====================
	// Safe shutdown
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigs
		fmt.Printf("\n")

		log.Printf("[Main] Attempting safe shutdown\n")
		a := time.Now()

		log.Printf("[Main] Shutting down HTTP server...\n")
		err := webServer.Close()
		if err != nil {
			log.Println(err)
		}

		log.Printf("[Main] Shutting down API server...\n")
		err = apiServer.Close()
		if err != nil {
			log.Println(err)
		}

		log.Printf("[Main] Signalling program exit...\n")
		b := time.Now().Sub(a)
		log.Printf("[Main] Took '%s' to fully shutdown modules\n", b.String())
		wg.Done()
	}()
	//
	//=====================

	wg.Wait()
	log.Println("[Main] Goodbye")
}

func check(err error) {
	if err != nil {
		panic(err)
	}
}
