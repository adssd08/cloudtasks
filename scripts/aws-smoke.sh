#!/bin/sh
set -eu

: "${AWS_ENDPOINT_URL:?set AWS_ENDPOINT_URL for this local smoke test}"

export AWS_ACCESS_KEY_ID=test
export AWS_SECRET_ACCESS_KEY=test
export AWS_DEFAULT_REGION=us-east-1
export AWS_EC2_METADATA_DISABLED=true

unset AWS_SESSION_TOKEN AWS_SECURITY_TOKEN AWS_PROFILE AWS_DEFAULT_PROFILE

aws \
    --endpoint-url "$AWS_ENDPOINT_URL" \
    --region "$AWS_DEFAULT_REGION" \
    --no-cli-pager \
    s3api list-buckets