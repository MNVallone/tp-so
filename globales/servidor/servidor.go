package servidor

import (
	"encoding/json"
	"log"
	"net/http"
	"log/slog"
	"fmt"
)

// ------ DECLARACION DE ESTRUCTURAS ------ //
type Mensaje struct {
	Mensaje string `json:"mensaje"`
}
/*
type Paquete struct {
	Valores []string `json:"valores"`
}
*/
type Paquete struct {
	Valores  []string `json:"valores"`
	UnNumero int      `json:"un_numero"`
}

type PCB struct{
	PID int `json:"pid"`
	ESTADO string `json:"estado"`
	ESPACIO_EN_MEMORIA int `json:"espacio_en_memoria"`
}

// ------ DECODIFICAR PAQUETE GENERICO ------ //
func decodificarPaquete[T any](w http.ResponseWriter ,r *http.Request, estructura *T) (T){
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
func RecibirPaquetesCpu(w http.ResponseWriter, r *http.Request) { // prueba cliente kernel y servidor memoria
	paquete := PCB{} 
	paquete = decodificarPaquete(w,r,&paquete)

	slog.Info("Me llego un mensaje del CPU")
	log.Printf("%+v\n", paquete)

	w.WriteHeader(http.StatusOK)
	w.Write([]byte("ok"))
}

func RecibirPaquetesKernel(w http.ResponseWriter, r *http.Request) { // prueba cliente kernel y servidor memoria
	paquete := Mensaje{} 
	paquete = decodificarPaquete(w,r,&paquete)

	slog.Info("Me llego un mensaje del kernel")
	log.Printf("%+v\n", paquete)

	w.WriteHeader(http.StatusOK)
	w.Write([]byte("ok"))
}

func RecibirPaquetesMemoria(w http.ResponseWriter, r *http.Request) { // prueba cliente kernel y servidor memoria
	paquete := Mensaje{} 
	paquete = decodificarPaquete(w,r,&paquete)

	slog.Info("Me llego un mensaje del memoria")
	log.Printf("%+v\n", paquete)

	w.WriteHeader(http.StatusOK)
	w.Write([]byte("ok"))
}

func RecibirPaquetesIO(w http.ResponseWriter, r *http.Request) { // prueba cliente kernel y servidor memoria
	paquete := Paquete{} 
	paquete = decodificarPaquete(w,r,&paquete)

	slog.Info("Me llego un mensaje del IO")
	log.Printf("%+v\n", paquete)

	w.WriteHeader(http.StatusOK)
	w.Write([]byte("ok"))
}

/*
func RecibirPaquetes(w http.ResponseWriter, r *http.Request) { // request estructura
	decoder := json.NewDecoder(r.Body)
	var paquete Paquete
	err := decoder.Decode(&paquete)
	if err != nil {
		log.Printf("error al decodificar mensaje: %s\n", err.Error())
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte("error al decodificar mensaje"))
		return
	}

	log.Println("me llego un paquete de un cliente")
	log.Printf("%+v\n", paquete)

	w.WriteHeader(http.StatusOK)
	w.Write([]byte("ok"))


func RecibirPaquetes(w http.ResponseWriter, r *http.Request) {
	paquete := Paquete{} 
	paquete = decodificarPaquete(w,r,&paquete)

	slog.Info("me llego un paquete de un cliente")
	log.Printf("%+v\n", paquete)

	w.WriteHeader(http.StatusOK)
	w.Write([]byte("ok"))
}

func RecibirMensaje(w http.ResponseWriter, r *http.Request) {
	decoder := json.NewDecoder(r.Body)
	var mensaje Mensaje
	err := decoder.Decode(&mensaje)
	if err != nil {
		log.Printf("Error al decodificar mensaje: %s\n", err.Error())
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte("Error al decodificar mensaje"))
		return
	}

	log.Println("Me llego un mensaje de un cliente")
	log.Printf("%+v\n", mensaje)

	w.WriteHeader(http.StatusOK)
	w.Write([]byte("ok"))
}
*/

