package main

import (
	"fmt"
	"globales"
	"log/slog"
	"memoria/utils"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"strconv"
	"syscall"
)

func main() {

	var rutaConfig string

	// ------ CONFIGURACIONES ------ //

	_, currentFile, _, _ := runtime.Caller(0)    // devuelve ruta absoluta del .go actual
	utils.RutaModulo = filepath.Dir(currentFile) // obtiene el directorio del archivo

	if len(os.Args) < 2 {
		slog.Error("Falta el argumento de configuración")
		os.Exit(1)
	}

	rutaConfig = filepath.Join(utils.RutaModulo, "configs", os.Args[1])

	utils.ClientConfig = utils.IniciarConfiguracion(rutaConfig)
	utils.InicializarMemoria()

	// ------ LOGGING ------ //
	globales.ConfigurarLogger("memoria.log", utils.ClientConfig.LOG_LEVEL)
	slog.Info("Iniciando módulo Memoria", "puerto", utils.ClientConfig.PORT_MEMORY)

	if utils.ClientConfig == nil {
		slog.Error("No se pudo crear el config")
	}

	// ------ INICIALIZACION DE VARIABLES ------ //
	puerto_memoria := ":" + strconv.Itoa(utils.ClientConfig.PORT_MEMORY)

	mux := http.NewServeMux()

	// ------ INICIALIZACION DEL SERVIDOR ------ //

	mux.HandleFunc("/kernel/inicializar_proceso", utils.InicializarProceso)
	mux.HandleFunc("/kernel/suspender_proceso", utils.SuspenderProceso)
	mux.HandleFunc("/kernel/dessuspender_proceso", utils.DesSuspenderProceso)
	mux.HandleFunc("/kernel/finalizar_proceso", utils.FinalizarProceso)
	mux.HandleFunc("/kernel/dump_de_proceso", utils.DumpearProceso)

	mux.HandleFunc("/cpu/handshake", utils.AtenderHandshakeCPU)
	mux.HandleFunc("/cpu/leer_pagina", utils.LeerPaginaCompleta)
	mux.HandleFunc("/cpu/buscar_instruccion", utils.DevolverInstruccion)
	mux.HandleFunc("/cpu/leer_direccion", utils.LeerDireccion)
	mux.HandleFunc("/cpu/escribir_direccion", utils.EscribirDireccion)
	mux.HandleFunc("/cpu/obtener_marco", utils.ObtenerMarco)
	mux.HandleFunc("/cpu/escribir_pagina", utils.EscribirPaginaCompleta)

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	go escucharPeticiones(puerto_memoria, mux)

	<-sigChan // Esperar a recibir una señal
	//slog.Debug(fmt.Sprintf("Memoria contigua: %x ", utils.MemoriaDeUsuario))
	//DebugSwapCompleto()
	slog.Info("Cerrando modulo memoria ...")
}

func escucharPeticiones(puerto string, mux *http.ServeMux) {
	err := http.ListenAndServe(puerto, mux)
	if err != nil {
		slog.Error(fmt.Sprintf("Error al iniciar el servidor: %s", err.Error()))
		//panic(err)
	}
}
