package devprodvalidations

import (
	"log"
)

type TestStruct struct {
	Aaa string
}

type SomeStruct struct{}

func (BLA *TestStruct) PublicFunction() { // want "Receiver name does not comply with single letter rule"

}

func (t *TestStruct) unnecessaryNilCheck() error { // want "Function have unnecessary nil check at end"
	err := t.someThingReturnsError()

	if err != nil {
		return err
	}

	return nil
}

func (t *TestStruct) correctNilCheck() error {
	err := t.someThingReturnsError()

	return err
}

func (t *TestStruct) someThingReturnsError() error {
	return nil
}

func (t *TestStruct) paramName(i SomeStruct) { // want "Single character parameter name does not match with type"

}

func (t *TestStruct) ShouldNotHaveSecond() {

}

func (BLA TestStruct) priva() { // want "Receiver name does not comply with single letter rule"

}

func (t *TestStruct) hasLogCalls() {
	log.Println("something") // want "Don't use `log` package to log, use loggrus"
}
