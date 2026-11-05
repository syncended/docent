package symbols

import (
	"context"
	"testing"
)

func TestExtractHCL(t *testing.T) {
	src := `
terraform {
  required_version = ">= 1.5"
}

provider "aws" {
  region = var.region
}

variable "region" {
  type    = string
  default = "eu-west-1"
}

locals {
  bucket_prefix = "app-${var.region}"
  tags          = { team = "core" }
}

resource "aws_s3_bucket" "logs" {
  bucket = "${local.bucket_prefix}-logs"

  lifecycle {
    prevent_destroy = true
  }
}

data "aws_iam_policy_document" "read" {
  statement {
    actions = ["s3:GetObject"]
  }
}

module "network" {
  source = "./modules/network"
}

output "bucket_arn" {
  value = aws_s3_bucket.logs.arn
}

resource aws_instance web {
  ami = "ami-123"
}
`
	checkExtract(t, "hcl", src, []string{
		"region variable",
		"bucket_prefix variable",
		"tags variable",
		"logs object",
		"read object",
		"network namespace",
		"bucket_arn constant",
		"web object",
	}, []string{
		"terraform", "aws", "required_version", "type", "default", "team",
		"bucket", "prevent_destroy", "actions", "source", "value", "ami",
		"aws_s3_bucket", "aws_iam_policy_document", "aws_instance",
	})
}

func TestExtractHCLNameWithoutQuotes(t *testing.T) {
	src := "variable \"region\" {}\n"
	syms, err := Extract(context.Background(), "hcl", []byte(src))
	if err != nil {
		t.Fatal(err)
	}
	if len(syms) != 1 || src[syms[0].Start:syms[0].End] != "region" {
		t.Errorf("got %+v", syms)
	}
}
