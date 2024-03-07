package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/gorilla/websocket"
	"github.com/joho/godotenv"
)

var interrupt = make(chan os.Signal, 1)

func main() {
	err := godotenv.Load()
	if err != nil {
		log.Fatal("環境変数の読み込みエラー:", err)
	}

	signal.Notify(interrupt, syscall.SIGINT, syscall.SIGTERM)

	misskeyHost := os.Getenv("MISSKEY_HOST")
	misskeyApiKey := os.Getenv("MISSKEY_API_KEY")
	misskeyStreamingEndpoint := fmt.Sprintf("wss://%s/streaming?i=%s", misskeyHost, misskeyApiKey)

	bouyomiChanHost := os.Getenv("BOUYOMI_CHAN_HOST")
	bouyomiChanWebSocketURL := fmt.Sprintf("ws://%s/TalkAPI/", bouyomiChanHost)

	channelName := os.Getenv("MISSKEY_CHANNEL_NAME")

	conn, _, err := websocket.DefaultDialer.Dial(misskeyStreamingEndpoint, nil)
	if err != nil {
		log.Fatal("Misskey Streaming APIに接続できませんでした:", err)
	}
	defer conn.Close()

	bouyomiConn, err := connectToBouyomiChan(bouyomiChanWebSocketURL)
	if err != nil {
		log.Fatal("BouyomiChanに接続できませんでした:", err)
	}
	defer bouyomiConn.Close()

	go handleInterrupt(conn)

	connectToChannel(conn, channelName)
	readMisskeyStreaming(conn, bouyomiConn)
}

func handleInterrupt(conn *websocket.Conn) {
	<-interrupt
	fmt.Println("プログラムを終了します...")
	conn.Close()
	os.Exit(0)
}

func connectToChannel(conn *websocket.Conn, channel string) {
	connectMessage := map[string]interface{}{
		"type": "connect",
		"body": map[string]interface{}{
			"channel": channel,
		},
	}

	err := conn.WriteJSON(connectMessage)
	if err != nil {
		log.Fatal("チャンネルに接続できませんでした:", err)
	}
}

func readMisskeyStreaming(conn *websocket.Conn, bouyomiConn *websocket.Conn) {
	for {
		_, message, err := conn.ReadMessage()
		if err != nil {
			log.Println("Misskey Streaming APIからのメッセージの読み取りエラー:", err)
			return
		}

		var msg map[string]interface{}
		if err := json.Unmarshal(message, &msg); err != nil {
			log.Println("メッセージのJSON解析エラー:", err)
			continue
		}

		handleMisskeyMessage(msg, bouyomiConn)
	}
}

func handleMisskeyMessage(msg map[string]interface{}, bouyomiConn *websocket.Conn) {
	msgType, ok := msg["type"].(string)
	if !ok {
		log.Println("メッセージのタイプが不明です")
		return
	}

	switch msgType {
	case "channel":
		handleChannelMessage(msg, bouyomiConn)
	default:
		log.Printf("未知のメッセージタイプ: %s\n", msgType)
	}
}

func handleChannelMessage(msg map[string]interface{}, bouyomiConn *websocket.Conn) {
	body, ok := msg["body"].(map[string]interface{})
	if !ok {
		log.Println("メッセージのボディが不明です")
		return
	}

	channelID, ok := body["id"].(string)
	if !ok {
		log.Println("チャンネルIDが不明です")
		return
	}

	if channelID != "example" {
		log.Println("対象のチャンネルではありません")
		return
	}

	channelType, ok := body["type"].(string)
	if !ok {
		log.Println("メッセージのタイプが不明です")
		return
	}

	switch channelType {
	case "note":
		handleNoteMessage(body, bouyomiConn)
	default:
		log.Printf("未知のチャンネルメッセージタイプ: %s\n", channelType)
	}
}

func handleNoteMessage(body map[string]interface{}, bouyomiConn *websocket.Conn) {
	noteBody, ok := body["body"].(map[string]interface{})
	if !ok {
		log.Println("Noteメッセージのボディが不明です")
		return
	}

	text, ok := noteBody["text"].(string)
	if !ok {
		log.Println("Noteメッセージの本文が不明です")
		return
	}

	fmt.Printf("新しい投稿: %s\n", text)

	err := bouyomiConn.WriteJSON(map[string]interface{}{
		"Command":   "Talk",
		"Text":      text,
		"VoiceType": 0,
		"Volume":    -1,
		"Speed":     -1,
		"Tone":      -1,
		"Encoding":  0,
	})
	if err != nil {
		log.Println("BouyomiChanへのメッセージ送信エラー:", err)
	}
}

func connectToBouyomiChan(url string) (*websocket.Conn, error) {
	conn, _, err := websocket.DefaultDialer.Dial(url, nil)
	if err != nil {
		return nil, fmt.Errorf("BouyomiChanに接続できませんでした: %v", err)
	}
	return conn, nil
}
