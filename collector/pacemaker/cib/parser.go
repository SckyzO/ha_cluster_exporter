package cib

import (
	"context"
	"encoding/xml"
	"os/exec"
	"time"

	"github.com/pkg/errors"
)

type Parser interface {
	Parse() (Root, error)
}

type cibAdminParser struct {
	cibAdminPath string
	timeout      time.Duration
}

func (p *cibAdminParser) Parse() (Root, error) {
	var CIB Root
	ctx, cancel := context.WithTimeout(context.Background(), p.timeout)
	defer cancel()

	cibXML, err := exec.CommandContext(ctx, p.cibAdminPath, "--query", "--local").Output()
	if err != nil {
		return CIB, errors.Wrap(err, "error while executing cibadmin")
	}

	err = xml.Unmarshal(cibXML, &CIB)
	if err != nil {
		return CIB, errors.Wrap(err, "could not parse cibadmin status from XML")
	}

	return CIB, nil
}

func NewCibAdminParser(cibAdminPath string, timeout time.Duration) *cibAdminParser {
	return &cibAdminParser{cibAdminPath, timeout}
}
