package main

import (
	"globales"
	"globales/servidor"
	"cpu/utils"
	"log"
	"strconv"
)

func main() {
	// ------ LOGGING ------ //
	globales.ConfigurarLogger("cpu.log") // configurar logger

	// ------ CONFIGURACIONES ------ //
	utils.ClientConfig = utils.IniciarConfiguracion("config.json")
	if utils.ClientConfig == nil {
		log.Fatalf("No se pudo crear el config")
	}

	// ------ INICIALIZACION DE VARIABLES ------ //
	puerto := ":" + strconv.Itoa(utils.ClientConfig.PORT_CPU)
	ip_memoria := utils.ClientConfig.IP_MEMORY
	puerto_memoria := utils.ClientConfig.PORT_MEMORY
	ip_kernel := utils.ClientConfig.IP_KERNEL
	puerto_kernel := utils.ClientConfig.PORT_KERNEL

	log.Printf("El puerto es %s", puerto)

	// ------ INICIALIZACION DEL CLIENTE ------ //
	pcb := servidor.PCB{
		PID: 1120,
		ESTADO : "Hola desde el cpu",
		ESPACIO_EN_MEMORIA : 1024,
	}

	globales.GenerarYEnviarPaquete(&pcb, ip_memoria, puerto_memoria)
	globales.GenerarYEnviarPaquete(&pcb, ip_kernel, puerto_kernel)
}
