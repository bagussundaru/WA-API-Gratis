package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"time"
)

type WebhookMessage struct {
	From    string `json:"from"`
	Message struct {
		ID   string `json:"id"`
		Text string `json:"text"`
	} `json:"message"`
}

type SendMessage struct {
	Phone          string `json:"phone"`
	Message        string `json:"message"`
	ReplyMessageID string `json:"reply_message_id"`
}

type SaldoResponse struct {
	ClientSaldo struct {
		Nama    string `json:"nama"`
		Saldo   string `json:"saldo"`
		Updated string `json:"updated"`
	} `json:"clientsaldo"`
	Code int    `json:"code"`
	Msg  string `json:"msg"`
}

type MutasiResponse struct {
	ClientMutasi struct {
		Nama   string `json:"nama"`
		Mutasi []struct {
			Tgl        string `json:"tgl"`
			Total      string `json:"total"`
			Jenis      string `json:"jenis"`
			Keterangan string `json:"keterangan"`
		} `json:"mutasi"`
	} `json:"clientmutasi"`
	Code int    `json:"code"`
	Msg  string `json:"msg"`
}

type ClientResponse struct {
	Code int    `json:"code"`
	Msg  string `json:"msg"`
}

const tabunganToken = "xxxxxxx"
const wapiuser = "admin"
const wapipass = "xxxxxxx"

// Clean WhatsApp Number
func cleanWhatsAppNumber(whatsappNumber string) string {
	// Split the WhatsApp number by "@" to get the part before "@"
	parts := strings.Split(whatsappNumber, "@")
	if len(parts) < 2 {
		return whatsappNumber // return the original number if format is unexpected
	}

	// Extract the numeric part before "@"
	numberPart := parts[0]

	// Remove any device part (e.g., ":54")
	numberPart = strings.Split(numberPart, ":")[0]

	// Remove non-numeric characters if any (such as spaces or other symbols)
	cleanNumber := strings.ReplaceAll(numberPart, " ", "")

	return cleanNumber
}

func webhookHandler(w http.ResponseWriter, r *http.Request) {
	// Read the body of the request
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	log.Printf("Received request body: %s\n", body)

	var msg WebhookMessage
	err = json.Unmarshal(body, &msg)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	fmt.Printf("Received message: %s from: %s Msg ID : %s\n", msg.Message.Text, msg.From, msg.Message.ID)

	// Process the received message and determine the response
	cleanedNumber := cleanWhatsAppNumber(msg.From)

	fmt.Printf("CleanNumber : %s", cleanedNumber)

	//log the messageID for debugging
	log.Printf("Message ID in webhookHandler: %s", msg.Message.ID)

	processMessage(cleanedNumber, msg.From, msg.Message.Text, msg.Message.ID)
}

func processMessage(clientno string, sender string, receivedMessage string, messageID string) {

	receivedMessage = strings.ToLower(receivedMessage)

	var responseMessage string

	switch receivedMessage {
	case "saldo", "ceksaldo":
		responseMessage = cekSaldo(clientno)
	case "mutasi", "cekmutasi":
		responseMessage = cekMutasi(clientno)
	case "help", "bantuan":
		responseMessage = getHelpMessage(clientno)
	default:
		aiReply, err := forwardToN8n(WebhookMessage{
			From: sender,
			Message: struct {
				ID   string `json:"id"`
				Text string `json:"text"`
			}{
				ID:   messageID,
				Text: receivedMessage,
			},
		})
		if err != nil {
			responseMessage = "⚠ Error: Gagal menghubungi AI agent."
		} else {
			responseMessage = aiReply
		}
	}

	log.Printf("Message ID in processMessage: %s\n", messageID)

	// Send the response back to the sender
	sendResponse(sender, responseMessage, messageID)
}

func cekSaldo(clientno string) string {
	url := "https://tabungan.musahefiz.id/api/ceksaldo"
	query := map[string]string{
		"telp":  clientno,
		"token": tabunganToken,
	}

	fmt.Printf("Cek No : %s", clientno)

	jsonData, err := json.Marshal(query)
	if err != nil {
		log.Printf("Error marshalling saldo query JSON: %v\n", err)
		return "Error processing your request."
	}

	// Create an HTTP client with a longer timeout
	client := &http.Client{
		Timeout: 30 * time.Second, // Increase the timeout to 30 seconds
	}

	resp, err := client.Post(url, "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		log.Printf("Error querying saldo: %v\n", err)
		if os.IsTimeout(err) {
			return "⚠ Error: Connection to the server timed out. Please try again later."
		}
		return "⚠ Error: Unable to connect to the server. Please try again later."
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Printf("Error reading saldo response: %v\n", err)
		return "⚠ Error: Unable to read the server response. Please try again later."
	}

	var result map[string]interface{}
	if err := json.Unmarshal(body, &result); err != nil {
		log.Printf("Error unmarshalling response: %v\n", err)
		return "⚠ Error: Unable to process the server response. Please try again later."
	}

	// Print the JSON response
	log.Printf("Saldo Response: %v\n", result)

	var saldoResponse SaldoResponse
	err = json.Unmarshal(body, &saldoResponse)
	if err != nil {
		log.Printf("Error unmarshalling saldo response: %v\n", err)
		return "⚠ Error: Unable to process the server response. Please try again later."
	}

	// If not registered
	if saldoResponse.Code == 0 {
		return saldoResponse.Msg
	}

	return fmt.Sprintf("_Assalamu'alaikum Wr. Wb,_\n\nBpk/Ibu %s,\n\n```Saldo Anda : %s\nTransaksi Terakhir : %s```\n\nInfo lebih lanjut hub\n📞 *Nur Indah* 081326825016\n\nTerima Kasih.\n\n_Wassalamu'alaikum Wr. Wb_",
		saldoResponse.ClientSaldo.Nama,
		saldoResponse.ClientSaldo.Saldo,
		saldoResponse.ClientSaldo.Updated)
}

func cekMutasi(clientno string) string {
	url := "https://tabungan.musahefiz.id/api/cekmutasi"
	query := map[string]string{
		"telp":  clientno,
		"token": tabunganToken,
	}

	jsonData, err := json.Marshal(query)
	if err != nil {
		log.Printf("Error marshalling mutasi query JSON: %v\n", err)
		return "Error processing your request."
	}

	// Create an HTTP client with a longer timeout
	client := &http.Client{
		Timeout: 30 * time.Second, // Increase the timeout to 30 seconds
	}

	resp, err := client.Post(url, "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		log.Printf("Error querying mutasi: %v\n", err)
		if os.IsTimeout(err) {
			return "⚠ Error: Connection to the server timed out. Please try again later."
		}
		return "⚠ Error: Unable to connect to the server. Please try again later."
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Printf("Error reading mutasi response: %v\n", err)
		return "⚠ Error: Unable to read the server response. Please try again later."
	}

	var mutasiResponse MutasiResponse
	err = json.Unmarshal(body, &mutasiResponse)
	if err != nil {
		log.Printf("Error unmarshalling mutasi response: %v\n", err)
		return "⚠ Error: Unable to process the server response. Please try again later."
	}

	// If not registered
	if mutasiResponse.Code == 0 {
		return mutasiResponse.Msg
	}

	var mts string
	for _, mutasi := range mutasiResponse.ClientMutasi.Mutasi {
		mts += fmt.Sprintf("%s: %s (%s)\n", mutasi.Tgl, mutasi.Total, mutasi.Jenis)
	}

	return fmt.Sprintf("_Assalamu'alaikum Wr. Wb,_\n\nBpk/Ibu %s,\n\nBerikut mutasi 10 transaksi terakhir : \n```%s```\n\nInfo lebih lanjut hub\n📞 *Nur Indah* 081326825016\n\n_Wassalamu'alaikum Wr. Wb_",
		mutasiResponse.ClientMutasi.Nama,
		mts)
}

func getHelpMessage(clientno string) string {
	url := "https://tabungan.musahefiz.id/api/info"
	query := map[string]string{
		"telp":  clientno,
		"token": tabunganToken,
	}

	fmt.Printf("Cek No : %s\n", clientno)

	jsonData, err := json.Marshal(query)
	if err != nil {
		log.Printf("Error marshalling client info query JSON: %v\n", err)
		return "Error processing your request."
	}

	// Create an HTTP client with a longer timeout
	client := &http.Client{
		Timeout: 30 * time.Second, // Increase the timeout to 30 seconds
	}

	resp, err := client.Post(url, "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		log.Printf("Error querying client: %v\n", err)
		return "⚠ Error: Unable to connect to the serve. Please try again later."
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Printf("Error reading client response: %v\n", err)
		return "⚠ Error: Unable to read the server response. Please try again later."
	}

	var result map[string]interface{}
	if err := json.Unmarshal(body, &result); err != nil {
		log.Printf("Error unmarshalling response: %v\n", err)
		return "⚠ Error: Unable to process the server response. Please try again later."
	}

	// Print the JSON response
	log.Printf("Client Response: %v\n", result)

	var clientResponse ClientResponse
	err = json.Unmarshal(body, &clientResponse)
	if err != nil {
		log.Printf("Error unmarshalling client response: %v\n", err)
		return "⚠ Error: Unable to process the server response. Please try again later."
	}

	// If not registered
	if clientResponse.Code == 0 {
		return clientResponse.Msg
	}

	return "Selamat Datang di Program Tabungan Umroh\n🕋 Musahefiz Blora\n\n*saldo* - _untuk cek saldo tabungan_\n*mutasi* - _untuk cek mutasi tabungan_\n\n*Bank Transfer:*\n```Bank Syariah Indonesia\nKode Bank 451\nNo. Rek 7264943811\na.n. Tabungan Musahefiz Blora``` \n\n\nInfo lebih lanjut hub\n📞 *Nur Indah* 081326825016\n\nTerima Kasih."
}

func sendResponse(to string, message string, replyMessageID string) {
	// Clean the phone number to remove the device part
	cleanedPhone := cleanWhatsAppNumber(to)

	// Log the cleaned phone number and new ReplyMessageID for debugging
	log.Printf("Cleaned phone number: %s\n", cleanedPhone)
	log.Printf("New ReplyMessageID in sendResponse: %s\n", replyMessageID)

	sendMessage := SendMessage{
		Phone:          cleanedPhone,
		Message:        message,
		ReplyMessageID: replyMessageID,
	}

	jsonData, err := json.Marshal(sendMessage)
	if err != nil {
		log.Printf("Error marshalling response JSON: %v\n", err)
		return
	}

	// Send the response via the appropriate endpoint
	responseURL := "http://" + wapiuser + ":" + wapipass + "@192.168.37.3:8000/send/message"

	// Set up the request with the appropriate headers
	req, err := http.NewRequest("POST", responseURL, bytes.NewBuffer(jsonData))
	if err != nil {
		log.Printf("Error creating new request: %v\n", err)
		return
	}
	req.Header.Set("Content-Type", "application/json")

	// Create an HTTP client with a longer timeout
	client := &http.Client{
		Timeout: 30 * time.Second, // Increase the timeout to 30 seconds
	}
	resp, err := client.Do(req)
	if err != nil {
		log.Printf("Error sending response message: %v\n", err)
		return
	}
	defer resp.Body.Close()

	// Log the response status and body for debugging
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Printf("Error reading response body: %v\n", err)
		return
	}

	log.Printf("Response status: %s\n", resp.Status)
	log.Printf("Response body: %s\n", respBody)
	log.Printf("Sent response message: %s to %s\n", message, to)
}

func forwardToN8n(msg WebhookMessage) (string, error) {
	n8nURL := "http://localhost:5678/webhook/ai-agent" // ganti dengan URL webhook n8n kamu

	jsonData, err := json.Marshal(msg)
	if err != nil {
		return "", err
	}

	resp, err := http.Post(n8nURL, "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	// Asumsikan n8n membalas dengan {"reply":"jawaban dari AI"}
	var result map[string]string
	if err := json.Unmarshal(body, &result); err != nil {
		return "", err
	}

	return result["reply"], nil
}

func main() {
	http.HandleFunc("/webhooks", webhookHandler)

	log.Println("Starting server on :8080...")
	log.Fatal(http.ListenAndServe(":8080", nil))
}
