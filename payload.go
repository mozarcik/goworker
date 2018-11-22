package goworker

import "strings"
import "errors"

import "github.com/google/uuid"

type JobUUID struct {
	UUID uuid.UUID
}

type Payload struct {
	UUID  *JobUUID      `json:"uuid"`
	Class string        `json:"class"`
	Args  []interface{} `json:"args"`
}

func (self *JobUUID) UnmarshalJSON(b []byte) error {
	s := strings.Trim(string(b), "\"")
	var err error
	uuid, err := uuid.Parse(s)
	if err != nil {
		return errors.New("Could not parse UUID " + err.Error())
	}
	self.UUID = uuid
	return nil
}

func (self *JobUUID) String() string {
	return self.UUID.String()
}
