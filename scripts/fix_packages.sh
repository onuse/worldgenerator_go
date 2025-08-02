#!/bin/bash

# Fix package declarations in subdirectories

# Fix opengl package files
for file in rendering/opengl/*.go; do
    if [ -f "$file" ]; then
        sed -i '1s/^package rendering$/package opengl/' "$file"
    fi
done

# Fix textures package files  
for file in rendering/textures/*.go; do
    if [ -f "$file" ]; then
        sed -i '1s/^package rendering$/package textures/' "$file"
    fi
done

# Fix overlay package files
for file in rendering/opengl/overlay/*.go; do
    if [ -f "$file" ]; then
        sed -i '1s/^package rendering$/package overlay/' "$file"
    fi
done

# Fix shaders package files
for file in rendering/opengl/shaders/*.go; do
    if [ -f "$file" ]; then
        sed -i '1s/^package rendering$/package shaders/' "$file"
    fi
done

echo "Package declarations fixed!"