package utils

import (
	"os"
	"encoding/json"
	"log"
)

type Config struct {
	IP_MEMORY 				string `json:"ip_memory"`
	IP_KERNEL 				string `json:"ip_kernel"`
	PORT_CPU			int `json:"port_cpu"`
	PORT_MEMORY 			int `json:"port_memory"`
	PORT_KERNEL 			int `json:"port_kernel"`
	TLB_ENTRIES 	string `json:"tlb_entries"`
	TLB_REPLACEMENT 			string `json:"tlb_replacement"`
	CACHE_ENTRIES 					int `json:"cache_entries"`
	CACHE_REPLACEMENT 		int `json:"cache_replacement"`
	CACHE_DELAY 				string `json:"cache_delay"`
	LOG_LEVEL 				string `json:"log_level"`
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
