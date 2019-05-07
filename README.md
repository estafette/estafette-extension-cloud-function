# estafette-extension-cloud-function
This extension can be used to create and deploy a cloud function

# Usage

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