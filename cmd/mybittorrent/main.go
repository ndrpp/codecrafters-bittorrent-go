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

	lengthStr := bencodedString[i : firstColonIndex]
	length, err := strconv.Atoi(lengthStr)
	if err != nil {
        return "", fmt.Errorf("Failed to decode string length: %v", err), 0
	}
	i += firstColonIndex - i + length
	return bencodedString[firstColonIndex+1 : firstColonIndex+1+length], nil, firstColonIndex - i + length + 1
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

func decodeList(i int, bencodedString string) (interface{}, error, int) {
	result := make([]any, 0, 10)
    var charLen int
	for j := i; j < len(bencodedString); j++ {
		switch {
		case unicode.IsDigit(rune(bencodedString[j])):
			str, err, length := decodeString(j, bencodedString)
            //fmt.Println("str: ", str)
			if err != nil {
				return "", err, 0
			}
			result = append(result, str)
			j += length

		case bencodedString[j] == 'i':
			num, err, length := decodeInteger(j, bencodedString)
            //fmt.Println("num: ", num)
			if err != nil {
				return "", err, 0
			}
			result = append(result, num)
			j += length - 1

        case bencodedString[j] == 'e':
            charLen = j - i
            //fmt.Println("charLen: ", charLen)
            break

        case bencodedString[j] == 'l' && j != i:
            list, err, length := decodeList(j, bencodedString)
			if err != nil {
				return "", err, 0
			}
			result = append(result, list)
			j += length 

        default:
            continue
		}
	}

	return result, nil, charLen
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
	} else {
		fmt.Println("Unknown command: " + command)
		os.Exit(1)
	}
}
