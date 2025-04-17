package main

import (
	"globales"
	"globales/servidor"
	"memoria/utils"
	"log/slog"
	"net/http"
	"strconv"
	"os"
	"os/signal"
	"syscall"
	"fmt"
)

func main() {
	// ------ CONFIGURACIONES ------ //
	utils.ClientConfig = utils.IniciarConfiguracion("config.json")

	// ------ LOGGING ------ //
	globales.ConfigurarLogger("memoria.log",utils.ClientConfig.LOG_LEVEL)
	slog.Info("Iniciando módulo Memoria", "puerto", utils.ClientConfig.PORT_MEMORY)

	if utils.ClientConfig == nil {
		slog.Error("No se pudo crear el config")
	}

	// ------ INICIALIZACION DE VARIABLES ------ //
	puerto_memoria := ":" + strconv.Itoa(utils.ClientConfig.PORT_MEMORY)
	//log_level := utils.ClientConfig.LOG_LEVEL

	mux := http.NewServeMux()

	// ------ INICIALIZACION DEL SERVIDOR ------ //
	mux.HandleFunc("/paqueteCPU", servidor.RecibirPaquetesCpu) // TODO: implementar para CPU
	mux.HandleFunc("/paqueteKernel", servidor.RecibirPaquetesKernel) // TODO: implementar para Kernel

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	err := http.ListenAndServe(puerto_memoria, mux)
	if err != nil {
		slog.Error(fmt.Sprintf("Error al iniciar el servidor: %s", err.Error()))
		//panic(err)
	}

	<-sigChan // Esperar a recibir una señal
	
	slog.Info("Cerrando modulo memoria ...")
}
