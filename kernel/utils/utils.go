package utils

import (
	"os"
	"encoding/json"
	"log"
)

type Config struct {
	IP_MEMORY 				string `json:"ip_memory"`
	PORT_MEMORY 			int `json:"port_memory"`
	PORT_KERNEL 			int `json:"port_kernel"`
	SCHEDULER_ALGORITHM 	string `json:"sheduler_algorithm"`
	NEW_ALGORITHM 			string `json:"new_algorithm"`
	ALPHA 					int `json:"alpha"`
	SUSPENSION_TIME 		int `json:"suspension_time"`
	LOG_LEVEL 				string `json:"log_level"`
}

var ClientConfig *Config

type Paquete struct {
	Valores string `json:"valores"`
}

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
