package main

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
)

func logAlert(w http.ResponseWriter, r *http.Request) {
	body, _ := ioutil.ReadAll(r.Body)
	f, _ := os.OpenFile("alerts.log", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	defer f.Close()
	f.WriteString(string(body) + "\n")
	fmt.Println("Logged alert:", string(body))
}

func main() {
	http.HandleFunc("/log-alert", logAlert)
	http.ListenAndServe(":5001", nil)
}
