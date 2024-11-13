#!/bin/sh
 
echo Namespace from config is $NAMESPACE

# Check if `nginx-deployment` exists
if kubectl get deployment nginx-deployment --namespace=${NAMESPACE} &> /dev/null; then
    echo "nginx-deployment exists"
else
    echo "nginx-deployment does not exist"
    exit 1
fi

# Check if `nginx-deployment` is running

if kubectl get pods --namespace=${NAMESPACE} | grep nginx-deployment | grep Running &> /dev/null; then
    echo "nginx-deployment is running"
else
    echo "nginx-deployment is not running"
    exit 1
fi

# Check if `nginx-deployment` is exposed

if kubectl get svc nginx-deployment --namespace=${NAMESPACE} &> /dev/null; then
    echo "nginx-deployment is exposed"
else
    echo "nginx-deployment is not exposed"
    exit 1
fi
