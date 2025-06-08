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
	utils.InicializarMemoria()

	// ------ LOGGING ------ //
	globales.ConfigurarLogger("memoria.log", utils.ClientConfig.LOG_niveles)
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
	mux.HandleFunc("/kernel/destruir_proceso", utils.DestruirProceso)
	mux.HandleFunc("/kernel/dump_de_proceso", utils.DumpearProceso)
	mux.HandleFunc("/kernel/crear_proceso", utils.CrearProceso)
	mux.HandleFunc("/cpu/buscar_instruccion", utils.DevolverInstruccion)
	mux.HandleFunc("/cpu/leer_direccion", utils.LeerDireccion)
	mux.HandleFunc("/cpu/escribir_direccion", utils.EscribirDireccion)

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	TablaPaginas := utils.CrearTablaPaginas(1, utils.ClientConfig.NUMBER_OF_LEVELS, utils.ClientConfig.ENTRIES_PER_PAGE)
	TablaPaginas2 := utils.CrearTablaPaginas(1, utils.ClientConfig.NUMBER_OF_LEVELS, utils.ClientConfig.ENTRIES_PER_PAGE)

	utils.ReservarMemoria(2115, TablaPaginas)
	utils.ReservarMemoria(1000, TablaPaginas2)

	var marcosAsignados1 []int
	var marcosAsignados2 []int

	utils.ObtenerMarcosAsignados(TablaPaginas, 1, &marcosAsignados1)
	utils.ObtenerMarcosAsignados(TablaPaginas2, 1, &marcosAsignados2)
	fmt.Println("Los marcos asignados al proceso son: ")
	fmt.Println(marcosAsignados1)
	fmt.Println(marcosAsignados2)

	go escucharPeticiones(puerto_memoria, mux)

	<-sigChan // Esperar a recibir una señal
	slog.Info("Cerrando modulo memoria ...")
}

func escucharPeticiones(puerto string, mux *http.ServeMux) {
	err := http.ListenAndServe(puerto, mux)
	if err != nil {
		slog.Error(fmt.Sprintf("Error al iniciar el servidor: %s", err.Error()))
		//panic(err)
	}
}
