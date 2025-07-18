package main

import (
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"runtime"
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
	LOG_LEVEL        string `json:"log_level"`
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
var local bool

var IP_CPU string
var IP_KERNEL string
var IP_MEMORIA string
var IP_IO string
var DUMP_PATH string
var SCRIPTS_PATH string

func main() {
	_, currentFile, _, _ := runtime.Caller(0)
	rutaArchivo = filepath.Dir(currentFile)
	var decision int = 0

	DUMP_PATH = filepath.Join(rutaArchivo, "memoria", "dump")
	SCRIPTS_PATH = filepath.Join(rutaArchivo, "globales", "archivos_prueba")

	for decision != 7 {
		fmt.Println("Cambiar IPs de modulo (escribir el numero): \n - 1 CPU \n - 2 Memoria \n - 3 Kernel \n - 4 IO \n - 5 Setear IPs \n - 6 Salir")
		fmt.Scan(&decision)
		switch decision {
		case 1:
			actualizarIPsCPU()
			break
		case 2:
			actualizarIPsMemoria()
			break
		case 3:
			actualizarIPsKernel()
			break
		case 4:
			actualizarIPsIO()
		case 5:
			setearIPs()
			break

		}
		fmt.Println("\033[H\033[2J")
	}
}

func setearIPs() {
	fmt.Println("Ingrese la IP de la CPU:")
	fmt.Scan(&IP_CPU)
	fmt.Println("Ingrese la IP del Kernel:")
	fmt.Scan(&IP_KERNEL)
	fmt.Println("Ingrese la IP de la Memoria:")
	fmt.Scan(&IP_MEMORIA)
	fmt.Println("Ingrese la IP de la IO:")
	fmt.Scan(&IP_IO)
}

func actualizarIPsIO() {
	configsPath := filepath.Join(rutaArchivo, "io", "configs")

	err := filepath.WalkDir(configsPath, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			log.Println(err)
			return nil // Opcional: continuar a pesar del error
		}
		if !d.IsDir() {
			go modificarConfigIO(path)
			//fmt.Println("Archivo: %s modificado", path)
		}
		return nil
	})
	if err != nil {
		log.Fatalf("Error al recorrer el directorio: %v", err)
	}
}

func actualizarIPsKernel() {
	configsPath := filepath.Join(rutaArchivo, "kernel", "configs")

	err := filepath.WalkDir(configsPath, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			log.Println(err)
			return nil // Opcional: continuar a pesar del error
		}
		if !d.IsDir() {
			go modificarConfigKernel(path)
			//fmt.Println("Archivo: %s modificado", path)
		}
		return nil
	})
	if err != nil {
		log.Fatalf("Error al recorrer el directorio: %v", err)
	}
}

func actualizarIPsCPU() {
	configsPath := filepath.Join(rutaArchivo, "cpu", "configs")

	err := filepath.WalkDir(configsPath, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			log.Println(err)
			return nil // Opcional: continuar a pesar del error
		}
		if !d.IsDir() {
			go modificarConfigCPU(path)
			//fmt.Println("Archivo: %s modificado", path)
		}
		return nil
	})
	if err != nil {
		log.Fatalf("Error al recorrer el directorio: %v", err)
	}
}

func actualizarIPsMemoria() {
	configsPath := filepath.Join(rutaArchivo, "memoria", "configs")

	err := filepath.WalkDir(configsPath, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			log.Println(err)
			return nil // Opcional: continuar a pesar del error
		}
		if !d.IsDir() {
			go modificarConfigMemoria(path)
			//fmt.Println("Archivo: %s modificado", path)
		}
		return nil
	})
	if err != nil {
		log.Fatalf("Error al recorrer el directorio: %v", err)
	}
}

func modificarConfigIO(path string) {
	nuevaConfig := ConfigIO{}
	configFile, err := os.OpenFile(path, os.O_RDONLY, 0644)
	if err != nil {
		fmt.Println("No se pudo abrir el archivo de configuración de CPU:", path)
		return
	}
	bytes, err := io.ReadAll(configFile)
	if err != nil {
		fmt.Println("No se pudo leer el archivo de configuración de CPU:", path)
		configFile.Close()
		return
	}
	configFile.Close()

	errDeco := json.Unmarshal(bytes, &nuevaConfig)
	//log.Printf("Modificando archivo de configuración de CPU: %v", nuevaConfig)
	if errDeco != nil {
		fmt.Println("No se pudo decodificar el archivo de configuración de CPU:", path)
		return
	}
	nuevaConfig.IP_KERNEL = IP_KERNEL
	nuevaConfig.IP_IO = IP_IO

	//log.Printf("Modificando archivo de configuración de CPU: %v", nuevaConfig)
	dataJson, _ := json.MarshalIndent(nuevaConfig, " ", " ")
	configFile, err = os.OpenFile(path, os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		fmt.Println("No se pudo abrir el archivo de configuración de CPU para escribir:", path)
		return
	}
	defer configFile.Close()
	configFile.Write(dataJson)
}

func modificarConfigKernel(path string) {
	nuevaConfig := ConfigKernel{}
	configFile, err := os.OpenFile(path, os.O_RDONLY, 0644)
	if err != nil {
		fmt.Println("No se pudo abrir el archivo de configuración de CPU:", path)
		return
	}
	bytes, err := io.ReadAll(configFile)
	if err != nil {
		fmt.Println("No se pudo leer el archivo de configuración de CPU:", path)
		configFile.Close()
		return
	}
	configFile.Close()

	errDeco := json.Unmarshal(bytes, &nuevaConfig)
	//log.Printf("Modificando archivo de configuración de CPU: %v", nuevaConfig)
	if errDeco != nil {
		fmt.Println("No se pudo decodificar el archivo de configuración de CPU:", path)
		return
	}
	nuevaConfig.IP_MEMORY = IP_MEMORIA
	nuevaConfig.IP_KERNEL = IP_KERNEL

	//log.Printf("Modificando archivo de configuración de CPU: %v", nuevaConfig)
	dataJson, _ := json.MarshalIndent(nuevaConfig, " ", " ")
	configFile, err = os.OpenFile(path, os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		fmt.Println("No se pudo abrir el archivo de configuración de CPU para escribir:", path)
		return
	}
	defer configFile.Close()
	configFile.Write(dataJson)
}

func modificarConfigMemoria(path string) {
	nuevaConfig := ConfigMemoria{}
	configFile, err := os.OpenFile(path, os.O_RDONLY, 0644)
	if err != nil {
		fmt.Println("No se pudo abrir el archivo de configuración de CPU:", path)
		return
	}
	bytes, err := io.ReadAll(configFile)
	if err != nil {
		fmt.Println("No se pudo leer el archivo de configuración de CPU:", path)
		configFile.Close()
		return
	}
	configFile.Close()

	errDeco := json.Unmarshal(bytes, &nuevaConfig)
	//log.Printf("Modificando archivo de configuración de CPU: %v", nuevaConfig)
	if errDeco != nil {
		fmt.Println("No se pudo decodificar el archivo de configuración de CPU:", path)
		return
	}
	nuevaConfig.IP_MEMORY = IP_MEMORIA
	nuevaConfig.DUMP_PATH = DUMP_PATH
	nuevaConfig.SCRIPTS_PATH = SCRIPTS_PATH

	//log.Printf("Modificando archivo de configuración de CPU: %v", nuevaConfig)
	dataJson, _ := json.MarshalIndent(nuevaConfig, " ", " ")
	configFile, err = os.OpenFile(path, os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		fmt.Println("No se pudo abrir el archivo de configuración de CPU para escribir:", path)
		return
	}
	defer configFile.Close()
	configFile.Write(dataJson)
}

func modificarConfigCPU(path string) {
	nuevaConfig := ConfigCPU{}
	configFile, err := os.OpenFile(path, os.O_RDONLY, 0644)
	if err != nil {
		fmt.Println("No se pudo abrir el archivo de configuración de CPU:", path)
		return
	}
	bytes, err := io.ReadAll(configFile)
	if err != nil {
		fmt.Println("No se pudo leer el archivo de configuración de CPU:", path)
		configFile.Close()
		return
	}
	configFile.Close()

	errDeco := json.Unmarshal(bytes, &nuevaConfig)
	//log.Printf("Modificando archivo de configuración de CPU: %v", nuevaConfig)
	if errDeco != nil {
		fmt.Println("No se pudo decodificar el archivo de configuración de CPU:", path)
		return
	}
	nuevaConfig.IP_MEMORY = IP_MEMORIA
	nuevaConfig.IP_KERNEL = IP_KERNEL
	nuevaConfig.IP_CPU = IP_CPU
	//log.Printf("Modificando archivo de configuración de CPU: %v", nuevaConfig)
	dataJson, _ := json.MarshalIndent(nuevaConfig, " ", " ")
	configFile, err = os.OpenFile(path, os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		fmt.Println("No se pudo abrir el archivo de configuración de CPU para escribir:", path)
		return
	}
	defer configFile.Close()
	configFile.Write(dataJson)
}
