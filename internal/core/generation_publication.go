package core

type ResourceGenerationPublication struct {
	Generation ResourceGeneration `json:"generation"`
	Candidate  PersistentResource `json:"candidate"`
	State      string             `json:"state"`
}
