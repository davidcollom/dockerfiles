#!/usr/bin/env bash
# set -e
# set -x

REPO=$1

# Ensure experimental flags enabled
exper=$(cat ${DOCKER_CONFIG:-~/.docker}/config.json | grep experimental | grep -q enabled )
if [[ $? != 0 ]]
then
    echo "Experimental cli flags required for multi arch building"
    exit 1
fi

# IMAGES=$(find * -maxdepth 1 -type d)
IMAGES=${@:2}

# Loop through all the directories
for img in $IMAGES; do
    echo "Building ${img}..."
    if [ -f ${img}/.skip ]; then
        echo "Skipping building of ${img} due to .skip file present"
        continue
    fi
    version=$(cat ${img}/VERSION 2>/dev/null)
    imgname="${REPO}/${img}:${version:-latest}"
    all_imgs=""
    dckr_images=$(find $img -iname 'Dockerfile*')
    add_images=$(find $img -iname 'image*')
    multi_arch=$(find $img -iname 'Dockerfile*' -or -iname 'image*' | wc -l | sed -e 's/ //g')

    if [[ $multi_arch == '1' ]]; then
        echo "Not a Multi-Arch directory - standard build/push"
        docker build --build-arg VERSION=${version:-latest} -t $imgname -f ${dckr_images[0]} $img/ | sed "s/^/\[$img\] /"
        docker push $imgname  | sed "s/^/\[$img\] /"
        continue
    fi

    echo $dckr_images
    # Loop through all Dockerfiles    
    for dckr_file in $dckr_images; do
        arch="${dckr_file##*.}"
        # Build Image
        docker build --pull --build-arg VERSION=${version:-latest} -t "${imgname}-${arch}" -f $dckr_file $img/ | sed "s/^/\[$img\] /"
        # Push Image (Needs to exist for creating manifests)
        docker push "${imgname}-${arch}" | sed "s/^/\[$img\] /"
        all_imgs+="${imgname}-${arch} "
    done
    for addimg in $add_images; do
        addimgname=$(cat $addimg)
        all_imgs+=" ${addimgname}"
    done

    # Create the Manifest 
    docker manifest create --amend ${imgname} ${all_imgs} | sed "s/^/\[$img\] /"

    # We need to loop back again
    # So that we can annotate the images with os/arch
    for dckr_file in $dckr_images; do
        arch="${dckr_file##*.}"
        docker manifest annotate ${imgname} "${imgname}-${arch}" --os linux --arch=${arch} | sed "s/^/\[$img\] /"
    done

    for add_img in $add_images; do
        arch="${add_img##*.}"
        addimgname=$(cat ${add_img})
        docker manifest annotate ${imgname} "${addimgname}" --os linux --arch=${arch} | sed "s/^/\[$img\] /"
    done
    # Finally push the manifest to the hub
    docker manifest push -p ${imgname} | sed "s/^/\[$img\] /"
done
