package main

import (
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode"
)

func decodeString(i int, bencodedString string) (string, error, int) {
	var firstColonIndex int
	for j := i; j < len(bencodedString); j++ {
		if bencodedString[j] == ':' {
			firstColonIndex = j
			break
		}
	}

	lengthStr := bencodedString[i:firstColonIndex]
	length, err := strconv.Atoi(lengthStr)
	if err != nil {
		return "", fmt.Errorf("Failed to decode string length: %v", err), 0
	}
	i += firstColonIndex - i + length
	return bencodedString[firstColonIndex+1 : firstColonIndex+1+length], nil, i
}

func bencodeString(str string) string {
	return fmt.Sprintf("%d:%s", len(str), str)
}

func decodeInteger(i int, bencodedString string) (int, error, int) {
	var numberLen int
	init := i
	for j := i; j < len(bencodedString); j++ {
		if bencodedString[j] == 'e' {
			numberLen = j - i
			break
		}
	}
	i += numberLen

	res, err := strconv.Atoi(bencodedString[init+1 : i])
	return res, err, numberLen + 1
}

func bencodeInteger(num int) string {
	return fmt.Sprintf("i%de", num)
}

func decodeList(i int, bencodedString string) ([]any, error, int) {
	result := make([]any, 0, 10)
	var charLen int
listLoop:
	for j := i; j < len(bencodedString); j++ {
		switch {
		case unicode.IsDigit(rune(bencodedString[j])):
			str, err, position := decodeString(j, bencodedString)
			if err != nil {
				return make([]any, 0), err, 0
			}
			result = append(result, str)
			j = position

		case bencodedString[j] == 'i':
			num, err, length := decodeInteger(j, bencodedString)
			if err != nil {
				return make([]any, 0), err, 0
			}
			result = append(result, num)
			j += length - 1

		case bencodedString[j] == 'e':
			charLen = j - i
			break listLoop

		case (bencodedString[j] == 'l' || bencodedString[j] == 'd') && j != i:
			list, err, length := decodeList(j, bencodedString)
			if err != nil {
				return make([]any, 0), err, 0
			}
			result = append(result, list)
			j += length

		default:
			continue
		}
	}

	return result, nil, charLen
}

func parseList(list []any) (map[string]any, error) {
	result := make(map[string]any, 10)
	for j := 0; j < len(list); j += 2 {
		v, ok := list[j].(string)
		if ok == false {
			return make(map[string]any, 0), fmt.Errorf("Key must be a string")
		}

		second, ok := list[j+1].([]any)
		if ok == false {
			result[v] = list[j+1]
		} else {
			if len(second)%2 != 0 {
				result[v] = second
			} else {
				result[v], _ = parseList(second)
			}
		}
	}
	return result, nil
}

func decodeDict(i int, bencodedString string) (map[string]any, error, int) {
	list, _, length := decodeList(i, bencodedString)
	res, err := parseList(list)
	return res, err, length
}

func decodeBencode(bencodedString string) (interface{}, error, int) {
	for i := 0; i < len(bencodedString); i++ {
		switch {
		case unicode.IsDigit(rune(bencodedString[i])):
			return decodeString(i, bencodedString)

		case bencodedString[i] == 'i':
			return decodeInteger(i, bencodedString)

		case bencodedString[i] == 'l':
			return decodeList(i, bencodedString)

		case bencodedString[i] == 'd':
			return decodeDict(i, bencodedString)

		default:
			return "", fmt.Errorf("Unsupported"), 0
		}
	}
	return "", fmt.Errorf("Invalid bencodedString"), 0
}

func main() {
	command := os.Args[1]

	switch command {
	case "decode":
		bencodedValue := os.Args[2]

		decoded, err, _ := decodeBencode(bencodedValue)
		if err != nil {
			fmt.Println(err)
			return
		}

		jsonOutput, _ := json.Marshal(decoded)
		fmt.Println(string(jsonOutput))

	case "info":
		fp := os.Args[2]

		content, err := os.ReadFile(fp)
		if err != nil {
			fmt.Println(err)
			return
		}
		res, err, _ := decodeDict(0, string(content))
		if err != nil {
			fmt.Println(err)
			return
		}
		tracker := res["announce"]
		info := map[string]any(res["info"].(map[string]any))
		length := info["length"]
		pieces := []byte(info["pieces"].(string))
		pieceLength := info["piece length"]
		bencodedInfo, err := bencode(info)
		if err != nil {
			fmt.Println(err)
			return
		}
		sha := hashInfo(bencodedInfo)
		hexedSha := hex.EncodeToString(sha)

		fmt.Fprintln(os.Stdout, "Tracker URL:", tracker)
		fmt.Fprintln(os.Stdout, "Length:", length)
		fmt.Fprintln(os.Stdout, "Info Hash:", hexedSha)

		fmt.Fprintln(os.Stdout, "Piece Length:", pieceLength)
		fmt.Fprintln(os.Stdout, "Piece Hashes:")
		for i := 0; i < len(pieces); i += 20 {
			fmt.Fprintln(os.Stdout, hex.EncodeToString(pieces[i:i+20]))
		}

	case "peers":
		fp := os.Args[2]

		content, err := os.ReadFile(fp)
		if err != nil {
			fmt.Println(err)
			return
		}
		res, err, _ := decodeDict(0, string(content))
		if err != nil {
			fmt.Println(err)
			return
		}
		tracker := res["announce"]
		info := map[string]any(res["info"].(map[string]any))
		length := info["length"]
		bencodedInfo, err := bencode(info)
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}
		sha := hashInfo(bencodedInfo)

		peers := getPeers(sha, tracker, length)
		for i := 0; i < len(peers); i += 6 {
			fmt.Fprintf(os.Stdout, fmt.Sprintf("%d.%d.%d.%d:%d\n", int(peers[i]), int(peers[i+1]), int(peers[i+2]), int(peers[i+3]), int(peers[i+4])<<8|int(peers[i+5])))
		}

	case "handshake":
		fp := os.Args[2]
		address := os.Args[3]

		content, err := os.ReadFile(fp)
		if err != nil {
			fmt.Println(err)
			return
		}
		res, err, _ := decodeDict(0, string(content))
		if err != nil {
			fmt.Println(err)
			return
		}
		info := map[string]any(res["info"].(map[string]any))
		bencodedInfo, err := bencode(info)
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}
		sha := hashInfo(bencodedInfo)
		conn, err := net.Dial("tcp", address)
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}
		defer conn.Close()
		reserved := make([]byte, 8)
		buf := make([]byte, 512)

		conn.Write([]byte{0b00010011})
		conn.Write([]byte("BitTorrent protocol"))
		conn.Write(reserved)
		conn.Write(sha)
		conn.Write([]byte("00112233445566778899"))

		size, err := conn.Read(buf)
		if err != nil {
			fmt.Println(err)
			return
		}
		fmt.Fprintln(os.Stdout, fmt.Sprintf("Peer ID: %x", buf[48:int(size)]))

	case "download_piece":
		fp := os.Args[2]
		content, err := os.ReadFile(fp)
		if err != nil {
			fmt.Println(err)
			return
		}
		res, err, _ := decodeDict(0, string(content))
		if err != nil {
			fmt.Println(err)
			return
		}
		tracker := res["announce"]
		info := map[string]any(res["info"].(map[string]any))
		length := info["length"]
		//pieces := []byte(info["pieces"].(string))
		//pieceLength := info["piece length"]
		bencodedInfo, err := bencode(info)
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}
		sha := hashInfo(bencodedInfo)

		peers := getPeers(sha, tracker, length)
		address := fmt.Sprintf("%d.%d.%d.%d:%d", int(peers[0]), int(peers[1]), int(peers[2]), int(peers[3]), int(peers[4])<<8|int(peers[5]))

		conn, err := net.Dial("tcp", address)
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}
		defer conn.Close()
		reserved := make([]byte, 8)

		conn.Write([]byte{0b00010011})
		conn.Write([]byte("BitTorrent protocol"))
		conn.Write(reserved)
		conn.Write(sha)
		conn.Write([]byte("00112233445566778899"))

		for {
			buf := make([]byte, 256)
			size, err := conn.Read(buf)
			if err != nil {
				fmt.Println(err)
				return
			}

			fmt.Fprintln(os.Stdout, "received:", buf[:size])
			switch {
			case buf[4] == 5: //bitfield
				conn.Write([]byte{0, 0, 0, 1, 2})

			case buf[4] == 1: //unchoke
				// TODO

			default:
			}
		}

	default:
		fmt.Println("Unknown command: " + command)
		os.Exit(1)

	}
}

func getPeers(sha []byte, tracker, length any) []byte {
	client := &http.Client{
		Timeout: time.Duration(time.Duration.Seconds(5)),
	}
	req, err := http.NewRequest("GET", tracker.(string), nil)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	q := req.URL.Query()
	q.Add("info_hash", string(sha))
	q.Add("peer_id", "11111111111111111111")
	q.Add("port", "6881")
	q.Add("uploaded", "0")
	q.Add("downloaded", "0")
	q.Add("left", strconv.Itoa(length.(int)))
	q.Add("compact", "1")
	req.URL.RawQuery = q.Encode()

	resp, err := client.Do(req)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	body, err := io.ReadAll(resp.Body)
	defer resp.Body.Close()
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	decodedBody, err, _ := decodeDict(0, string(body))
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	return []byte(decodedBody["peers"].(string))
}

func hashInfo(bencodedInfo string) []byte {
	hasher := sha1.New()
	_, err := hasher.Write([]byte(bencodedInfo))
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to sha1 hash the info: %s\n", err)
		os.Exit(1)
	}
	return hasher.Sum(nil)
}

func bencodeList(arr []any) string {
	var b strings.Builder

	b.Write([]byte("l"))
	for _, value := range arr {
		switch v := value.(type) {
		case string:
			str := bencodeString(v)
			b.Write([]byte(str))

		case int:
			num := bencodeInteger(v)
			b.Write([]byte(num))

		case []any:
			list := bencodeList(v)
			b.Write([]byte(list))

		default:
			return ""
		}
	}
	b.Write([]byte("e"))

	return b.String()
}

func bencode(dict map[string]any) (string, error) {
	var b strings.Builder
	keys := make([]string, len(dict))

	for k := range dict {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	b.Write([]byte("d"))
	for _, k := range keys[4:] { //for some reason first 4 elements are nil??
		b.Write([]byte(bencodeString(k)))
		switch v := dict[k].(type) {
		case string:
			str := bencodeString(v)
			b.Write([]byte(str))

		case int:
			num := bencodeInteger(v)
			b.Write([]byte(num))

		case []interface{}:
			list := bencodeList(v)
			b.Write([]byte(list))

		default:
			return "", fmt.Errorf("unexpected type %T", v)
		}
	}
	b.Write([]byte("e"))

	return b.String(), nil
}
