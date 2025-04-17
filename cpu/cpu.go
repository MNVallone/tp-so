package main

import (
	"fmt"
	"globales"
	"globales/servidor"
	"cpu/utils"
	"log/slog"
	"strconv"
	"os"
	"os/signal"
	"syscall"
)

func main() {
	// ------ CONFIGURACIONES ------ //
	utils.ClientConfig = utils.IniciarConfiguracion("config.json")

	// ------ LOGGING ------ //
	globales.ConfigurarLogger("cpu.log", utils.ClientConfig.LOG_LEVEL) // configurar logger

	if utils.ClientConfig == nil {
		slog.Error("No se pudo crear el config")
	}

	// ------ INICIALIZACION DE VARIABLES ------ //
	puerto := ":" + strconv.Itoa(utils.ClientConfig.PORT_CPU)
	ip_memoria := utils.ClientConfig.IP_MEMORY
	puerto_memoria := utils.ClientConfig.PORT_MEMORY
	ip_kernel := utils.ClientConfig.IP_KERNEL
	puerto_kernel := utils.ClientConfig.PORT_KERNEL

	slog.Info(fmt.Sprintf("El puerto es %s", puerto))

	// ------ INICIALIZACION DEL CLIENTE ------ //
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	pcb := servidor.PCB{
		PID: 1120,
		ESTADO : "Hola desde el cpu",
		ESPACIO_EN_MEMORIA : 1024,
	}

	globales.GenerarYEnviarPaquete(&pcb, ip_memoria, puerto_memoria, "/paqueteCPU")
	globales.GenerarYEnviarPaquete(&pcb, ip_kernel, puerto_kernel, "/paqueteCPU")

	<-sigChan 

	slog.Info("Cerrando modulo CPU ...")
}
