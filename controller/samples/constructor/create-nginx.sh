#!/bin/sh
 
echo Namespace from config is $NAMESPACE

# Create nginx pod
kubectl create deployment nginx-deployment \
    --image=nginx:latest \
    --namespace=${NAMESPACE}

# Expose nginx pod
kubectl expose deployment nginx-deployment \
    --port=80 \
    --type=NodePort \
    --namespace=${NAMESPACE}