package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"unicode"
	//bencode "github.com/jackpal/bencode-go" // Available if you need it!
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
			list, _, length := decodeList(i, bencodedString)
			res, err := parseList(list)
			return res, err, length

		default:
			return "", fmt.Errorf("Unsupported"), 0
		}
	}
	return "", fmt.Errorf("Invalid bencodedString"), 0
}

func main() {
	command := os.Args[1]

	if command == "decode" {
		bencodedValue := os.Args[2]

		decoded, err, _ := decodeBencode(bencodedValue)
		if err != nil {
			fmt.Println(err)
			return
		}

		jsonOutput, _ := json.Marshal(decoded)
		fmt.Println(string(jsonOutput))
	} else if command == "info" {
		fp := os.Args[2]

		content, err := os.ReadFile(fp)
		if err != nil {
			fmt.Println(err)
			return
		}
		list, _, _ := decodeList(0, string(content))
		res, err := parseList(list)
		if err != nil {
			fmt.Println(err)
			return
		}
		tracker := res["announce"]
		info := map[string]any(res["info"].(map[string]any))
		length := info["length"]
		fmt.Fprintln(os.Stdout, "Tracker URL:", tracker)
		fmt.Fprintln(os.Stdout, "Length:", length)

	} else {
		fmt.Println("Unknown command: " + command)
		os.Exit(1)
	}
}
