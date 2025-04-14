package main

import (
	"globales"
	"globales/servidor"
	"kernel/utils"
	"log"
	"net/http"
	"strconv"
)

func main() {
	// ------ LOGGING ------ //
	globales.ConfigurarLogger("kernel.log") // configurar logger

	// ------ CONFIGURACIONES ------ //
	utils.ClientConfig = utils.IniciarConfiguracion("config.json")
	if utils.ClientConfig == nil {
		log.Fatalf("No se pudo crear el config")
	}

	// ------ INICIALIZACION DE VARIABLES ------ //
	puerto_memoria := utils.ClientConfig.PORT_MEMORY
	puerto_kernel := ":" + strconv.Itoa(utils.ClientConfig.PORT_KERNEL)
	ip_memoria := utils.ClientConfig.IP_MEMORY

	mux := http.NewServeMux()

	
	// ------ INICIALIZACION DEL SERVIDOR ------ // Comentar para probar cliente
	
	// mux.HandleFunc("/paquete", servidor.RecibirPaquetes)
	mux.HandleFunc("/paquete", servidor.RecibirPaquetesCpu) //TODO: implementar para CPU
	//mux.HandleFunc("/paquete", servidor.RecibirPaquetes) //TODO: implementar para IO
	log.Printf("Servidor escuchando en el puerto %s", puerto_kernel)

	err := http.ListenAndServe(puerto_kernel, mux)
	if err != nil {
		log.Fatalf("Error al iniciar el servidor: %s", err.Error())
		//panic(err)
	}

	// ------ INICIALIZACION DEL CLIENTE ------ //
	mensaje := servidor.Mensaje{
		Mensaje : "Hola desde el kernel",
	}

	globales.GenerarYEnviarPaquete(&mensaje, ip_memoria, puerto_memoria)
}

