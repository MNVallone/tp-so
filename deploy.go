package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
)

type ConfigMemoria struct {
	PORT_MEMORY      int    `json:"port_memory"`
	IP_MEMORY        string `json:"ip_memory"`
	MEMORY_SIZE      int    `json:"memory_size"`
	PAGE_SIZE        int    `json:"page_size"`
	ENTRIES_PER_PAGE int    `json:"entries_per_page"`
	NUMBER_OF_LEVELS int    `json:"number_of_levels"`
	MEMORY_DELAY     int    `json:"memory_delay"`
	SWAPFILE_PATH    string `json:"swapfile_path"`
	SWAP_DELAY       int    `json:"swap_delay"`
	LOG_niveles      string `json:"log_niveles"`
	DUMP_PATH        string `json:"dump_path"`
	SCRIPTS_PATH     string `json:"scripts_path"`
}

type ConfigKernel struct {
	IP_MEMORY               string  `json:"ip_memory"`
	PORT_MEMORY             int     `json:"port_memory"`
	IP_KERNEL               string  `json:"ip_kernel"`
	PORT_KERNEL             int     `json:"port_kernel"`
	SCHEDULER_ALGORITHM     string  `json:"scheduler_algorithm"`
	READY_INGRESS_ALGORITHM string  `json:"ready_ingress_algorithm"`
	ALPHA                   float32 `json:"alpha"`
	INITIAL_ESTIMATE        float32 `json:"initial_estimate"`
	SUSPENSION_TIME         int     `json:"suspension_time"`
	LOG_LEVEL               string  `json:"log_level"`
}

type ConfigCPU struct {
	PORT_CPU          int    `json:"port_cpu"`
	IP_CPU            string `json:"ip_cpu"`
	PORT_MEMORY       int    `json:"port_memory"`
	IP_MEMORY         string `json:"ip_memory"`
	PORT_KERNEL       int    `json:"port_kernel"`
	IP_KERNEL         string `json:"ip_kernel"`
	TLB_ENTRIES       int    `json:"tlb_entries"`
	TLB_REPLACEMENT   string `json:"tlb_replacement"`
	CACHE_ENTRIES     int    `json:"cache_entries"`
	CACHE_REPLACEMENT string `json:"cache_replacement"`
	CACHE_DELAY       int    `json:"cache_delay"`
	LOG_LEVEL         string `json:"log_level"`
}

type ConfigIO struct {
	PORT_IO     int    `json:"port_io"`
	IP_IO       string `json:"ip_io"`
	IP_KERNEL   string `json:"ip_kernel"`
	PORT_KERNEL int    `json:"port_kernel"`
	LOG_LEVEL   string `json:"log_level"`
}

var rutaArchivo string

func main() {
	_, currentFile, _, _ := runtime.Caller(0)
	rutaArchivo = filepath.Dir(currentFile)
	var decision int = 0

	for decision != 7 {
		fmt.Println("Preparar entorno para prueba: \n  - 1 PLANI CORTO PLAZO \n  - 2 PLANI MYL PLAZO \n  - 3 SWAP \n  - 4 CACHE \n  - 5 TLB \n - 6 ESTABILIDAD GENERAL \n - 7 SALIR")
		fmt.Scan(&decision)
		switch decision {
		case 1:
			prepararPlaniCortoPlazo()
			break
		case 2:
			prepararPlaniMYLPlazo()
			break
		case 3:
			prepararSWAP()
			break
		case 4:
			prepararCACHE()
			break
		case 5:
			prepararTLB()
			break
		case 6:
			prepararEstabilidadGeneral()
			break

		}
	}

}

func modificarArchivoCPU(config *ConfigCPU) {
	filePath := filepath.Join(rutaArchivo, "cpu", "config.json")
	fmt.Println(filePath)

	configFile, err := os.OpenFile(filePath, os.O_CREATE, 0644)

	if err != nil {
		fmt.Errorf("No se pudo abrir el config de cpu")
	}

	dataJson, errJson := json.Marshal(config)

	if errJson != nil {
		fmt.Errorf("No se pudo codificar el config de cpu")
	}

	configFile.Write(dataJson)
	configFile.Close()

}


func prepararEstabilidadGeneral() {
	panic("unimplemented")
}

func prepararTLB() {
	panic("unimplemented")
}

func prepararCACHE() {
	panic("unimplemented")
}

func prepararSWAP() {
	panic("unimplemented")
}

func prepararPlaniMYLPlazo() {
	panic("unimplemented")
}

func prepararPlaniCortoPlazo() {
	configuracionCPU := ConfigCPU{
	PORT_CPU:          8004,
	IP_CPU:            "127.0.0.1",
	PORT_MEMORY:       8002,
	IP_MEMORY:         "127.0.0.1",
	PORT_KERNEL:       8001,
	IP_KERNEL:        "127.0.0.1",
	TLB_ENTRIES:       4,
	TLB_REPLACEMENT:   "LRU",
	CACHE_ENTRIES:    2,
	CACHE_REPLACEMENT: "CLOCK",
	CACHE_DELAY:       250,
	LOG_LEVEL:         "INFO",
	}
	modificarArchivoCPU(&configuracionCPU)
}

func ValidarArgumentosKernel() (string, int) {
	if len(os.Args) < 2 {
		fmt.Println("Error: Falta el archivo de pseudocódigo")
		fmt.Println("Uso: ./kernel [archivo_pseudocodigo] [tamanio_proceso]")
		os.Exit(1)
	}

	if len(os.Args) < 3 {
		fmt.Println("Error: Falta el tamaño del proceso")
		fmt.Println("Uso: ./kernel [archivo_pseudocodigo] [tamanio_proceso]")
		os.Exit(1)
	}

	rutaInicial := os.Args[1]
	tamanio, err := strconv.Atoi(os.Args[2])
	if err != nil {
		fmt.Println("Error: El tamaño del proceso debe ser un número entero")
		os.Exit(1)
	}
	return rutaInicial, tamanio
}
