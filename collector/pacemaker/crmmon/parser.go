package crmmon

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

type crmMonParser struct {
	crmMonPath string
	timeout    time.Duration
}

func (c *crmMonParser) Parse() (crmMon Root, err error) {
	ctx, cancel := context.WithTimeout(context.Background(), c.timeout)
	defer cancel()

	crmMonXML, err := exec.CommandContext(ctx, c.crmMonPath, "-X", "--inactive").Output()
	if err != nil {
		return crmMon, errors.Wrap(err, "error while executing crm_mon")
	}

	err = xml.Unmarshal(crmMonXML, &crmMon)
	if err != nil {
		return crmMon, errors.Wrap(err, "error while parsing crm_mon XML output")
	}

	return crmMon, nil
}

func NewCrmMonParser(crmMonPath string, timeout time.Duration) *crmMonParser {
	return &crmMonParser{crmMonPath, timeout}
}
