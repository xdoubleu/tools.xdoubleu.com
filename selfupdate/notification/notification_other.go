//go:build !darwin || !cgo

package notification

const supported = false

func requestAuth() {}

func post(string, string) {}
