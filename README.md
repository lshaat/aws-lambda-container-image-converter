## AWS Lambda Container Image Converter

This container image converter tool (img2lambda) repackages container images (such as Docker images) into AWS Lambda layers, and publishes them as new layer versions to Lambda.

The tool copies all files under '/opt' in the Docker image, maintaining the individual Docker image layers as individual Lambda layers.  The published layer ARNs will be stored in a file 'output/layers.json', which can be used as input when creating Lambda functions.  Each layer is named using a "namespace" prefix (like img2lambda or my-docker-image) and the SHA256 digest of the Docker image layer, in order to provide a way of tracking the provenance of the Lambda layer back to the Docker image that created it.

If a layer is already published to Lambda (same layer name, SHA256 digest, and size), it will not be published again.  Instead the existing layer version ARN will be written to the output file.

![img2lambda Demo](assets/demo.gif)

**Table of Contents**

<!-- toc -->

- [Usage](#usage)
- [Install](#install)
    + [Binaries](#binaries)
    + [From Source](#from-source)
- [Example](#example)
- [License Summary](#license-summary)

<!-- tocstop -->

## Usage

```
USAGE:
   img2lambda [options]

GLOBAL OPTIONS:
   --image value, -i value             Name of the source container image. For example, 'my-docker-image:latest'
   --region value, -r value            AWS region (default: "us-east-1")
   --output-directory value, -o value  Destination directory for command output (default: "./output")
   --layer-namespace value, -n value   Prefix for the layers published to Lambda (default: "img2lambda")
   --dry-run, -d                       Conduct a dry-run: Repackage the image, but only write the Lambda layers to local disk (do not publish to Lambda)
   --help, -h                          show help
```

## Install

#### Binaries

Download pre-built binaries from the [Releases Page](https://github.com/awslabs/aws-lambda-container-image-converter/releases).

#### From Source

```
$ mkdir -p $GOPATH/src/github.com/awslabs
$ git clone https://github.com/awslabs/aws-lambda-container-image-converter $GOPATH/src/github.com/awslabs/aws-lambda-container-image-converter
$ cd !$
$ make
$ ./bin/local/img2lambda --help
```

## Example

Build the example Docker image to create a PHP Lambda custom runtime:
```
cd example

docker build -t lambda-php .
```

The example PHP functions are also built into the example image, so they can be run with Docker:
```
docker run lambda-php hello '{"name": "World"}'

docker run lambda-php goodbye '{"name": "World"}'
```

Run the tool to create and publish Lambda layers that contain the PHP custom runtime:
```
./bin/local/img2lambda -i lambda-php:latest -r us-east-1
```

Create a PHP function that uses the layers:
```
cd function

zip hello.zip src/hello.php

aws lambda create-function \
    --function-name php-example-hello \
    --handler hello \
    --zip-file fileb://./hello.zip \
    --runtime provided \
    --role "arn:aws:iam::XXXXXXXXXXXX:role/service-role/LambdaPhpExample" \
    --region us-east-1 \
    --layers file://../output/layers.json
```

Finally, invoke the function:
```
aws lambda invoke \
    --function-name php-example-hello \
    --region us-east-1 \
    --log-type Tail \
    --query 'LogResult' \
    --output text \
    --payload '{"name": "World"}' hello-output.txt | base64 --decode

cat hello-output.txt
```

## License Summary

This sample code is made available under a modified MIT license. See the LICENSE file.
