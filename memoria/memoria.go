package main

import (
	"fmt"
	"globales"
	"globales/servidor"
	"log/slog"
	"memoria/utils"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
)

func main() {
	// ------ CONFIGURACIONES ------ //
	utils.ClientConfig = utils.IniciarConfiguracion("config.json")
	// memoriaContigua := make([]byte, utils.ClientConfig.MEMORY_SIZE)

	// ------ LOGGING ------ //
	globales.ConfigurarLogger("memoria.log", utils.ClientConfig.LOG_LEVEL)
	slog.Info("Iniciando módulo Memoria", "puerto", utils.ClientConfig.PORT_MEMORY)

	if utils.ClientConfig == nil {
		slog.Error("No se pudo crear el config")
	}

	// ------ INICIALIZACION DE VARIABLES ------ //
	puerto_memoria := ":" + strconv.Itoa(utils.ClientConfig.PORT_MEMORY)
	//log_level := utils.ClientConfig.LOG_LEVEL

	mux := http.NewServeMux()

	// ------ INICIALIZACION DEL SERVIDOR ------ //
	mux.HandleFunc("/cpu/paquete", utils.AtenderCPU)                  // TODO: implementar para CPU
	mux.HandleFunc("/kernel/paquete", servidor.RecibirPaquetesKernel) // TODO: implementar para Kernel
	mux.HandleFunc("/memoria/verificar_espacio", utils.VerificarEspacioDisponible)
	mux.HandleFunc("/memoria/reservar_espacio", utils.ReservarEspacio)
	mux.HandleFunc("/memoria/liberar_espacio", utils.LiberarEspacio)

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
