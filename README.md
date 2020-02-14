# estafette-extension-cloud-function
This extension can be used to create and deploy a cloud function, the default trigger is HTTP.

# Usage

HTTP trigger

```
releases:
    development:
        clone: true
        stages:
            deploy:
                image: extensions/cloud-function:stable
                runtime: go111
                memory: 256MB
```

Bucket trigger

```
releases:
    development:
        clone: true
        stages:
            deploy:
                image: extensions/cloud-function:stable
                runtime: go111
                memory: 256MB
                trigger: bucket
                triggerValue: bucketName
```
