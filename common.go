package main

import (
	"fmt"
	"os"
	"strconv"
)

func GetEnv(key, fallback string) string {
	if value, ok := os.LookupEnv(key); ok {
		return value
	}
	return fallback
}

func GetInt(strValue string, defaultValue int64) int64 {
	intValue, err := strconv.ParseInt(strValue, 10, 64)
	if err != nil {
		fmt.Printf("Incorrect int value, default value %+v will be used ", defaultValue)
		return defaultValue
	}
	return intValue
}
