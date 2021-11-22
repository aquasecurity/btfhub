#!/bin/bash

##
## This script IS SUPPOSED to be a big monolithic script.
## That's it: The tree should focus in arranging BTF data.
##

## Syntax: $0 [bionic|focal|centos{7,8}|fedora{29,30,31,32}]

basedir=$(dirname "${0}")
if [ "${basedir}" == "." ]; then
	basedir=$(pwd)
fi
basedir=$basedir/../archive/

##
## HELPER FUNCTIONS
##

exiterr() {
	echo ERROR: "${@}"
	exit 1
}

warn() {
	echo WARN: "${@}"
}

info() {
	echo INFO: "${@}"
}

###
### 1. UBUNTU (bionic, focal)
###

for arch in x86_64 arm64; do

for ubuntuver in bionic focal; do

    if [ "${1}" != "${ubuntuver}" ]; then
        continue
    fi

    case "${ubuntuver}" in
    "bionic")
        regex="(linux-image-unsigned-(4.15.0|5.4.0)-.*-(generic|azure)-dbgsym|linux-image-(4.15.0|5.4.0)-.*-aws-dbgsym)"
        ;;
    "focal")
        regex="(linux-image-unsigned-(5.4.0|5.8.0|5.11.0)-.*-(generic|azure)-dbgsym|linux-image-(5.4.0|5.8.0|5.11.0)-.*-aws-dbgsym)"
        ;;
    *)
        continue
        ;;
    esac

    case "${arch}" in
    "x86_64")
        altarch="amd64"
	;;
    "arm64")
	altarch="arm64"
	;;
    *)
	exiterr "could not find architecture"
	;;
    esac

    origdir=$(pwd)
    repository="http://ddebs.ubuntu.com"

    mkdir -p "${basedir}/ubuntu/${ubuntuver}"
    cd "${basedir}/ubuntu/${ubuntuver}/${arch}" || exiterr "no ${ubuntuver} dir found"

    wget http://ddebs.ubuntu.com/dists/${ubuntuver}/main/binary-${altarch}/Packages -O ${ubuntuver}
    wget http://ddebs.ubuntu.com/dists/${ubuntuver}-updates/main/binary-${altarch}/Packages -O ${ubuntuver}-updates

    [ ! -f ${ubuntuver} ] && exiterr "no ${ubuntuver} packages file found"
    [ ! -f ${ubuntuver}-updates ] && exiterr "no ${ubuntuver}-updates packages file found"

    grep -E '^(Package|Filename):' ${ubuntuver} | grep --no-group-separator -A1 -E "^Package: ${regex}" > temp
    grep -E '^(Package|Filename):' ${ubuntuver}-updates | grep --no-group-separator -A1 -E "Package: ${regex}" >> temp
    rm ${ubuntuver}; rm ${ubuntuver}-updates; mv temp packages

    grep "Package:" packages | sed 's:Package\: ::g' | sort | while read -r package; do

	    filepath=$(grep -A1 "${package}" packages | grep -v "^Package: " | sed 's:Filename\: ::g')
	    url="${repository}/${filepath}"
	    filename=$(basename "${filepath}")
	    version=$(echo "${filename}" | sed 's:linux-image-::g' | sed 's:-dbgsym.*::g' | sed 's:unsigned-::g')

	    echo URL: "${url}"
	    echo FILEPATH: "${filepath}"
	    echo FILENAME: "${filename}"
	    echo VERSION: "${version}"

	    if [ -f "${version}.btf.tar.xz" ] || [ -f "${version}.failed" ]; then
	    	info "file ${version}.btf already exists"
	    	continue
	    fi

	    if [ ! -f "${version}.ddeb" ]; then
	    	axel -4 -n 8 "${url}"
	    	mv "${filename}" "${version}.ddeb"
	    	if [ ! -f "${version}.ddeb" ]
	    	then
	    		warn "${version}.ddeb could not be downloaded"
	    		continue
	    	fi
	    fi

	    # extract vmlinux file from ddeb package
	    dpkg --fsys-tarfile "${version}.ddeb" | tar xvf - "./usr/lib/debug/boot/vmlinux-${version}" || \
	    {
	        warn "could not deal with ${version}, cleaning and moving on..."
	        rm -rf "${basedir}/ubuntu/${ubuntuver}/${arch}/usr"
	        rm -rf "${version}.ddeb"
		touch "${version}.failed"
	        continue
	    }

	    mv "./usr/lib/debug/boot/vmlinux-${version}" "./${version}.vmlinux" || \
	    {
	        warn "could not rename vmlinux ${version}, cleaning and moving on..."
	        rm -rf "${basedir}/ubuntu/${ubuntuver}/${arch}/usr"
	        rm -rf "${version}.ddeb"
		touch "${version}.failed"
	        continue

        }

	    rm -rf "${basedir}/ubuntu/${ubuntuver}/${arch}/usr"

	    pahole --btf_encode_detached "${version}.btf" "${version}.vmlinux"
	    # pahole "./${version}.btf" > "${version}.txt"
	    tar cvfJ "./${version}.btf.tar.xz" "${version}.btf"

        rm "${version}.ddeb"
	    rm "${version}.btf"
        # rm "${version}.txt"
	    rm "${version}.vmlinux"

    done

    pwd
    rm -f packages
    cd "${origdir}" >/dev/null || exit

done

done # arch

###
### 2. CENTOS (centos7, centos8)
###

for arch in x86_64 arm64; do

for centosver in centos7 centos8; do

    if [ "${1}" != "${centosver}" ]; then
        continue
    fi

    case "${arch}" in
    "x86_64")
        altarch="x86_64"
	;;
    "arm64")
	altarch="aarch64"
	;;
    *)
	exiterr "could not find architecture"
	;;
    esac

    centosrel=$1
    origdir=$(pwd)

    case "${centosver}" in
    "centos7")
      repository="http://mirror.facebook.net/centos-debuginfo/7/${altarch}/"
      ;;
    "centos8")
      repository="http://mirror.facebook.net/centos-debuginfo/8/${altarch}/Packages/"
      ;;
    esac

    regex="kernel-debuginfo-[0-9].*${altarch}.rpm"

    mkdir -p "${basedir}/centos/${centosver/centos/}"
    cd "${basedir}/centos/${centosver/centos/}/${arch}" || exiterr "no ${centosver} dir found"

    info "downloading ${repository} information"
    lynx -dump -listonly ${repository} | tail -n+4 > "${centosrel}"
    [[ ! -f ${centosrel} ]] && exiterr "no ${centosrel} packages file found"
    grep -E "${regex}" "${centosrel}" | awk '{print $2}' >temp
    mv temp packages
    rm "${centosrel}"

    sort packages | while read -r line; do

        url=${line}
        filename=$(basename "${line}")
        # shellcheck disable=SC2001
        version=$(echo "${filename}" | sed 's:kernel-debuginfo-\(.*\).rpm:\1:g')

        echo URL: "${url}"
        echo FILENAME: "${filename}"
        echo VERSION: "${version}"

        if [ -f "${version}.btf.tar.xz" ] || [ -f "${version}.failed" ]; then
          info "file ${version}.btf already exists"
          continue
        fi

        axel -4 -n 8 "${url}"
        mv "${filename}" "${version}.rpm"
        if [ ! -f "${version}.rpm" ]; then
          warn "${version}.rpm could not be downloaded"
          continue
        fi

        vmlinux=.$(rpmquery -qlp "${version}.rpm" 2>&1 | grep vmlinux)
        echo "INFO: extracting vmlinux from: ${version}.rpm"
        rpm2cpio "${version}.rpm" | cpio --to-stdout -i "${vmlinux}" > "./${version}.vmlinux" || \
        {
            warn "could not deal with ${version}, cleaning and moving on..."
	        rm -rf "${basedir}/centos/${centosver/centos/}/${arch}/usr"
	        rm -rf "${version}.rpm"
	        rm -rf "${version}.vmlinux"
		touch "${version}.failed"
	        continue
        }

        # generate BTF raw file from DWARF data
        echo "INFO: generating BTF file: ${version}.btf"
        pahole --btf_encode_detached "${version}.btf" "${version}.vmlinux"
        # pahole "${version}.btf" > "${version}.txt"
        tar cvfJ "./${version}.btf.tar.xz" "${version}.btf"

        rm "${version}.rpm"
        rm "${version}.btf"
        # rm "${version}.txt"
        rm "${version}.vmlinux"

    done

  rm -f packages
  cd "${origdir}" >/dev/null || exit

done

done #arch

###
### 3. Fedora
###

### fedora29-34

for arch in x86_64 arm64; do

for fedoraver in fedora29 fedora30 fedora31 fedora32 fedora33 fedora34; do

    if [ "${1}" != "${fedoraver}" ]; then
        continue
    fi

    case "${arch}" in
    "x86_64")
        altarch="x86_64"
	;;
    "arm64")
	altarch="aarch64"
	;;
    *)
	exiterr "could not find architecture"
	;;
    esac

    origdir=$(pwd)

    case "${fedoraver}" in

    "fedora29" | "fedora30" | "fedora31")
      repository01=https://archives.fedoraproject.org/pub/archive/fedora/linux/releases/"${fedoraver/fedora/}/Everything/${altarch}/debug/tree/Packages/k/"
      repository02=https://archives.fedoraproject.org/pub/archive/fedora/linux/updates/"${fedoraver/fedora/}/Everything/${altarch}/debug/Packages/k/"
      ;;
    "fedora32" | "fedora33" | "fedora34")
      repository01=https://dl.fedoraproject.org/pub/fedora/linux/releases/"${fedoraver/fedora/}/Everything/${altarch}/debug/tree/Packages/k/"
      repository02=https://dl.fedoraproject.org/pub/fedora/linux/releases/"${fedoraver/fedora/}/Everything/${altarch}/debug/tree/Packages/k/"
      ;;
    esac

    regex="kernel-debuginfo-[0-9].*${altarch}.rpm"

    mkdir -p "${basedir}/fedora/${fedoraver/fedora/}"
    cd "${basedir}/fedora/${fedoraver/fedora/}/${arch}" || exiterr "no ${fedoraver} dir found"

    info "downloading ${repository01} information"
    lynx -dump -listonly ${repository01} | tail -n+4 > ${fedoraver}
    info "downloading ${repository02} information"
    lynx -dump -listonly ${repository02} | tail -n+4 >> ${fedoraver}

    [[ ! -f ${fedoraver} ]] && exiterr "no ${fedoraver} packages file found"

    grep -E "${regex}" ${fedoraver} | awk '{print $2}' > temp
    mv temp packages ; rm ${fedoraver}

    sort packages | while read -r line; do

        url=${line}
        filename=$(basename "${line}")
        # shellcheck disable=SC2001
        version=$(echo "${filename}" | sed 's:kernel-debuginfo-\(.*\).rpm:\1:g')

        echo URL: "${url}"
        echo FILENAME: "${filename}"
        echo VERSION: "${version}"

        if [ -f "${version}.btf.tar.xz" ] || [ -f "${version}.failed" ]; then
          info "file ${version}.btf already exists"
          continue
        fi

        axel -4 -n 8 "${url}"
        mv "${filename}" "${version}.rpm"
        if [ ! -f "${version}.rpm" ]; then
          warn "${version}.rpm could not be downloaded"
          continue
        fi

        vmlinux=.$(rpmquery -qlp "${version}.rpm" 2>&1 | grep vmlinux)
        echo "INFO: extracting vmlinux from: ${version}.rpm"
        rpm2cpio "${version}.rpm" | cpio --to-stdout -i "${vmlinux}" > "./${version}.vmlinux" || \
        {
            warn "could not deal with ${version}, cleaning and moving on..."
	        rm -rf "${basedir}/fedora/${fedoraver/fedora/}/${arch}/usr"
	        rm -rf "${version}.rpm"
	        rm -rf "${version}.vmlinux"
		touch "${version}.failed"
	        continue
        }

        # generate BTF raw file from DWARF data
        echo "INFO: generating BTF file: ${version}.btf"
        pahole --btf_encode_detached "${version}.btf" "${version}.vmlinux"
        # pahole "${version}.btf" > "${version}.txt"
        tar cvfJ "./${version}.btf.tar.xz" "${version}.btf"

        rm "${version}.rpm"
        rm "${version}.btf"
        # rm "${version}.txt"
        rm "${version}.vmlinux"

    done

    rm -f packages
    cd "${origdir}" >/dev/null || exit

done

done #arch

exit 0
