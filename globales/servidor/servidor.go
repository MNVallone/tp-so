package servidor

import (
	"encoding/json"
	"fmt"
	"log"
	"log/slog"
	"net/http"
)

// ------ DECLARACION DE ESTRUCTURAS ------ //
type Mensaje struct {
	Mensaje string `json:"mensaje"`
}

type Paquete struct {
	Valores  []string `json:"valores"`
	UnNumero int      `json:"un_numero"`
}

type PCB struct {
	PID                int    `json:"pid"`
	ESTADO             string `json:"estado"`
	ESPACIO_EN_MEMORIA int    `json:"espacio_en_memoria"`
}

// ------ DECODIFICAR PAQUETE GENERICO ------ //
func DecodificarPaquete[T any](w http.ResponseWriter, r *http.Request, estructura *T) T {
	decoder := json.NewDecoder(r.Body)
	err := decoder.Decode(&estructura) //decodifica cualquier estructura que le pases por referencia sin importar su forma
	if err != nil {
		var zero T
		slog.Error(fmt.Sprintf("Error al decodificar mensaje: %s\n", err.Error()))
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte("Error al decodificar mensaje"))
		return zero // sujeto a modificaciones
	}
	return *estructura
}

// ------ RECIBIR ESTRUCTURA ------ //
func RecibirPaquetesCpu(w http.ResponseWriter, r *http.Request) PCB { // prueba cliente kernel y servidor memoria
	paquete := PCB{}
	paquete = DecodificarPaquete(w, r, &paquete)

	slog.Info("Me llego un mensaje del CPU")
	log.Printf("%+v\n", paquete)

	return paquete

	// Nota: En un futuro esto desaparece porque no es en este nivel que se tiene que mandar una respuesta a la CPU.
	// w.WriteHeader(http.StatusOK)
	// w.Write([]byte("ok"))
}

func RecibirPaquetesKernel(w http.ResponseWriter, r *http.Request) { // prueba cliente kernel y servidor memoria
	paquete := Mensaje{}
	paquete = DecodificarPaquete(w, r, &paquete)

	slog.Info("Me llego un mensaje del kernel")
	log.Printf("%+v\n", paquete)

	w.WriteHeader(http.StatusOK)
	w.Write([]byte("ok"))
}

func RecibirPaquetesMemoria(w http.ResponseWriter, r *http.Request) { // prueba cliente kernel y servidor memoria
	paquete := Mensaje{}
	paquete = DecodificarPaquete(w, r, &paquete)

	slog.Info("Me llego un mensaje del memoria")
	log.Printf("%+v\n", paquete)

	w.WriteHeader(http.StatusOK)
	w.Write([]byte("ok"))
}

func RecibirPaquetesIO(w http.ResponseWriter, r *http.Request) { // prueba cliente kernel y servidor memoria
	paquete := Paquete{}
	paquete = DecodificarPaquete(w, r, &paquete)

	slog.Info("Me llego un mensaje del IO")
	log.Printf("%+v\n", paquete)

	w.WriteHeader(http.StatusOK)
	w.Write([]byte("ok"))
}
