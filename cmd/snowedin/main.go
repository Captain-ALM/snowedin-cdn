package main

import (
	"fmt"
	"github.com/joho/godotenv"
	"gopkg.in/yaml.v3"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path"
	"snow.mrmelon54.xyz/snowedin/api"
	"snow.mrmelon54.xyz/snowedin/cdn"
	"snow.mrmelon54.xyz/snowedin/conf"
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
	y := time.Now()

	//Hold main thread till safe shutdown exit:
	wg := &sync.WaitGroup{}
	wg.Add(1)

	//Get working directory:

	cwdDir, err := os.Getwd()
	if err != nil {
		log.Println(err)
	}

	//Load environment file:

	err = godotenv.Load()
	if err != nil {
		log.Fatalln("Error loading .env file")
	}

	//Data directory processing:

	dataDir := os.Getenv("DIR_DATA")
	if dataDir == "" {
		dataDir = path.Join(cwdDir, ".data")
	}

	check(os.MkdirAll(dataDir, 0777))

	//Config loading:

	configFile, err := os.Open(path.Join(dataDir, "config.yml"))
	if err != nil {
		log.Fatalln("Failed to open config.yml")
	}

	var configYml conf.ConfigYaml
	groupsDecoder := yaml.NewDecoder(configFile)
	err = groupsDecoder.Decode(&configYml)
	if err != nil {
		log.Fatalln("Failed to parse config.yml:", err)
	}

	//Server definitions:

	log.Println("[Main] Starting up CDN server...")
	cdnServer := cdn.New(configYml)

	log.Printf("[Main] Starting up HTTP server on %s...\n", configYml.Listen.Web)
	webServer := web.New(cdnServer)

	var apiServer *http.Server
	if configYml.Listen.Api != "" {
		apiServer = api.New(cdnServer)
		log.Printf("[Main] Starting up API server on %s...\n", configYml.Listen.Api)
	}

	//=====================
	// Safe shutdown
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)

	//Startup complete:
	z := time.Now().Sub(y)
	log.Printf("[Main] Took '%s' to fully initialize modules\n", z.String())

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

		if apiServer != nil {
			log.Printf("[Main] Shutting down API server...\n")
			err = apiServer.Close()
			if err != nil {
				log.Println(err)
			}
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
