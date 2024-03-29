#!/usr/bin/env bash
set -e
# set -x

REPO=$1

# Ensure experimental flags enabled
exper=$(cat ${DOCKER_CONFIG:-~/.docker}/config.json | grep experimental | grep -q enabled )
if [[ $? != 0 ]]
then
    echo "Experimental cli flags required for multi arch building"
    exit 1
fi

# if x have been provided ONLY build those
if [[ ! -z $2 ]]; then
    IMAGES=(${@:2})
else
    IMAGES=$(find * -maxdepth 1 -type f -iname VERSION | cut -d'/' -f 1)
fi

echo "Processing: $IMAGES\n"

# Loop through all the directories
for img in $IMAGES; do
    echo "Building ${img}..."
    if [ -f ${img}/.skip ]; then
        echo "Skipping building of ${img} due to .skip file present"
        continue
    fi
    if [ ! -f ${img}/VERSION ]; then
        echo "No version file found for ${img} - one is required."
    fi

    version=$(cat ${img}/VERSION 2>/dev/null)
    imgname="${REPO}/${img}:${version:-latest}"
    all_imgs=""
    dckr_images=$(find $img -iname 'Dockerfile*')
    add_images=$(find $img -iname 'image*')
    multi_arch=$(find $img -iname 'Dockerfile*' -or -iname 'image*' | wc -l | sed -e 's/ //g')

    if [[ $multi_arch == '1' ]]; then
        echo "Not a Multi-Arch directory - standard build/push"
        docker build --pull --build-arg VERSION=${version:-latest} -t $imgname -f ${dckr_images[0]} $img/ | sed "s/^/\[$img\] /"
        docker push $imgname  | sed "s/^/\[$img\] /"
        continue
    fi

    echo $dckr_images
    # Loop through all Dockerfiles
    for dckr_file in $dckr_images; do
        arch="${dckr_file##*.}"
        # Build Image
        docker build --pull --build-arg VERSION=${version:-latest} -t "${imgname}-${arch}" -f $dckr_file $img/ | sed "s/^/\[$img@$arch\] /"
        # Push Image (Needs to exist for creating manifests)
        docker push "${imgname}-${arch}" | sed "s/^/\[$img@$arch\] /"
        all_imgs+="${imgname}-${arch} "
    done
    for add_img in $add_images; do
        addimgname=$(cat $add_img | sed "s/\[VERSION\]/${version}/")
        all_imgs+=" ${addimgname}"
    done

    # Create the Manifest
    docker manifest create --amend ${imgname} ${all_imgs} | sed "s/^/\[$img\] /"

    # We need to loop back again
    # So that we can annotate the images with os/arch
    for dckr_file in $dckr_images; do
        arch="${dckr_file##*.}"
        docker manifest annotate ${imgname} "${imgname}-${arch}" --os linux --arch=${arch} | sed "s/^/\[$img@$arch\] /"
    done

    for add_img in $add_images; do
        arch="${add_img##*.}"
        addimgname=$(cat ${add_img} | sed "s/\[VERSION\]/${version}/")
        docker manifest annotate ${imgname} "${addimgname}" --os linux --arch=${arch} | sed "s/^/\[$img@$arch\] /"
    done
    # Finally push the manifest to the hub
    docker manifest push -p ${imgname} | sed "s/^/\[$img\] /"
done
