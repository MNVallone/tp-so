package main

import (
	"globales"
	"globales/servidor"
	"memoria/utils"
	"log"
	"net/http"
	"strconv"
)

func main() {
	// ------ LOGGING ------ //
	globales.ConfigurarLogger("memoria.log") // configurar logger

	// ------ CONFIGURACIONES ------ //
	utils.ClientConfig = utils.IniciarConfiguracion("config.json")
	if utils.ClientConfig == nil {
		log.Fatalf("No se pudo crear el config")
	}

	// ------ INICIALIZACION DE VARIABLES ------ //
	puerto_memoria := ":" + strconv.Itoa(utils.ClientConfig.PORT_MEMORY)

	mux := http.NewServeMux()

	// ------ INICIALIZACION DEL SERVIDOR ------ //
	mux.HandleFunc("/paquete", servidor.RecibirPaquetesCpu) // TODO: implementar para CPU
	//mux.HandleFunc("/paquete", servidor.RecibirPaquetesKernel) // TODO: implementar para Kernel

	err := http.ListenAndServe(puerto_memoria, mux)
	if err != nil {
		log.Fatalf("Error al iniciar el servidor: %s", err.Error())
		//panic(err)
	}
}
