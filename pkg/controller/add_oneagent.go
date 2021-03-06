package controller

import (
	"github.com/Dynatrace/dynatrace-oneagent-operator/pkg/controller/oneagent"
)

func init() {
	// AddToManagerFuncs is a list of functions to create controllers and add them to a manager.
	AddToManagerFuncs = append(AddToManagerFuncs, oneagent.Add)
}
