package utils

import (
	"os"
	"encoding/json"
	"log"
)

type Config struct{
	IP_KERNEL 		string `json:"ip_kernel"`
    PORT_KERNEL 	int  `json:"port_kernel"`
    PORT_IO 		int `json:"port_io"`
    LOG_LEVEL 		string `json:"log_level"`
}

var ClientConfig *Config

func IniciarConfiguracion(filePath string) *Config {
	var config *Config
	configFile, err := os.Open(filePath)
	if err != nil {
		log.Fatal(err.Error())
	}
	defer configFile.Close()

	jsonParser := json.NewDecoder(configFile)
	jsonParser.Decode(&config)

	return config
}