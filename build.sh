#!/usr/bin/env bash
# set -e
# set -x

REPO=$1

# Ensure experimental flags enabled
exper=$(cat ${DOCKER_CONFIG:-~/.docker/config.json} | grep experimental | grep -q enabled )
if [[ $? != 0 ]]
then
    echo "Experimental cli flags required for multi arch building"
    exit 1
fi

IMAGES=$(find * -maxdepth 1 -type d)
# IMAGES=(unifi-exporter)
# IMAGES=(bind9-exporter)

# Loop through all the directories
for img in $IMAGES; do
    pushd $img
    if [ -f .skip ]; then
        echo "Skipping building of ${img} due to .skip file present"
        popd
        continue
    fi
    version=$(cat VERSION 2>/dev/null)
    imgname="${REPO}/${img}:${version:-latest}"
    all_imgs=""
    dckr_images=$(find . -iname 'Dockerfile*')
    add_images=$(find . -iname 'image*')
    multi_arch=$(find . -iname 'Dockerfile*' -or -iname 'image*' | wc -l | sed -e 's/ //g')

    if [[ $multi_arch == '1' ]]; then
        echo "Not a Multi-Arch directory - standard build/push"
        docker build --build-arg VERSION=${version:-latest} -t $imgname -f ${dckr_images[0]} .
        docker push $imgname
    else
        echo $dckr_images
        # Loop through all Dockerfiles    
        for dckr_file in $dckr_images; do
            arch="${dckr_file##*.}"
            # Build Image
            docker build --pull --build-arg VERSION=${version:-latest} -t "${imgname}-${arch}" -f $dckr_file .
            # Push Image (Needs to exist for creating manifests)
            docker push "${imgname}-${arch}"
            all_imgs+="${imgname}-${arch} "
        done
        for addimg in $add_images; do
            addimgname=$(cat $addimg)
            all_imgs+=" ${addimgname}"
        done

        # Create the Manifest 
        docker manifest create --amend ${imgname} ${all_imgs}

        # We need to loop back again
        # So that we can annotate the images with os/arch
        for dckr_file in $dckr_images; do
            arch="${dckr_file##*.}"
            docker manifest annotate ${imgname} "${imgname}-${arch}" --os linux --arch=${arch}
        done

        for add_img in $add_images; do
            arch="${add_img##*.}"
            addimgname=$(cat ${add_img})
            docker manifest annotate ${imgname} "${addimgname}" --os linux --arch=${arch}
        done
        # Finally push the manifest to the hub
        docker manifest push -p ${imgname}
    fi
    popd
done
