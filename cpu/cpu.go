package main

import (
	"fmt"
	"globales"
	"globales/servidor"
	"cpu/utils"
	"log"
	"net/http"
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
	mux := http.NewServeMux()
	fmt.Printf("El puerto es %s", puerto)
	// configuracion servidor CPU
	mux.HandleFunc("/paquete", servidor.RecibirPaquetes)

	// configuracion servidor IO

	err := http.ListenAndServe(puerto, mux)
	if err != nil {
		log.Fatalf("Error al iniciar el servidor: %s", err.Error())
		//panic(err)
	}
}
