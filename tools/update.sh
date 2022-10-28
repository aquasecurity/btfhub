#!/bin/bash

# vim: tabstop=4 shiftwidth=4 expandtab

echo "Updating BTF archives..."

##
## This script IS SUPPOSED to be a big monolithic script.
## That's it: The tree should focus in arranging BTF data.
##

## Syntax: $0 [bionic|focal|centos{7,8}|fedora{29,30,31,32}|amazon{1,2}|stretch|buster|bullseye|ol7]

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

for type in unsigned signed; do

    for arch in x86_64 arm64; do

        for ubuntuver in bionic focal; do

            if [ "${1}" != "${ubuntuver}" ]; then
                continue
            fi

            case "${ubuntuver}" in
                "bionic")
                    kernelversions=("4.15.0" "5.4.0")
                ;;
                "focal")
                    kernelversions=("5.4.0" "5.8.0" "5.11.0")
                ;;
                *)
                    continue
                ;;
            esac

            for kernelver in ${kernelversions[@]}; do

                if [ "$2" != "" ] && [ $2 != "$kernelver" ]; then
                    continue
                fi

                if [ "${type}" == "signed" ]; then
                    regex="linux-image-$kernelver-.*-(generic|azure|gke|gcp|aws)-dbgsym"
                else
                    regex="linux-image-unsigned-$kernelver-.*-(generic|azure|gke|gcp|aws)-dbgsym"
                fi

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

                mkdir -p "${basedir}/ubuntu/${ubuntuver}/${arch}"
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
                        curl -4 "${url}" -o ${version}.ddeb
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
                        if [ "${type}" == "signed" ]; then # maybe correct one is unsigned
                            touch "${version}.failed"
                        fi
                        continue
                    }

                    mv "./usr/lib/debug/boot/vmlinux-${version}" "./${version}.vmlinux" || \
                    {
                        warn "could not rename vmlinux ${version}, cleaning and moving on..."
                        rm -rf "${basedir}/ubuntu/${ubuntuver}/${arch}/usr"
                        rm -rf "${version}.ddeb"
                        touch "${version}.failed" # this is likely an error indeed
                        continue
                    }

                    rm -rf "./usr"

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

            done # kernelver

        done

    done # arch

done # type (signed/unsigned)

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

        mkdir -p "${basedir}/centos/${centosver/centos/}/${arch}"
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

            curl -4 "${url}" -o ${version}.rpm
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

        mkdir -p "${basedir}/fedora/${fedoraver/fedora/}/${arch}"
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

            curl -4 "${url}" -o ${version}.rpm
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

###
### 4. amazon (amazon1, amazon2)
###

for arch in x86_64 arm64; do

    for amazonver in amazon1 amazon2; do

        if [ "${1}" != "${amazonver}" ]; then
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
        case "${amazonver}" in
            "amazon1")
                if [ "${arch}" = "arm64" ]; then
                    continue
                fi
                ver="2018"
                repodataurl=http://packages.us-east-1.amazonaws.com/2018.03/updates/85446a8a5f59/debuginfo/x86_64
                repofilesurl="${repodataurl}"
                archiver="bzip2"
                archive="bz2"
            ;;
            "amazon2")
                ver="2"
                repository=https://amazonlinux-2-repos-us-east-2.s3.dualstack.us-east-2.amazonaws.com/2/core/latest/debuginfo/${altarch}/mirror.list
                archiver="gzip"
                archive="gz"
                info "downloading ${repository} mirror list"
                wget $repository
                info "downloading ${repository} information"
                repodataurl=$(head -1 mirror.list)
                repofilesurl=http://amazonlinux.us-east-1.amazonaws.com
                rm -f mirror.list
            ;;
            *)
                exiterr "unknown amazon linux version"
            ;;
        esac

        mkdir -p "${basedir}/amzn/${ver}/${arch}"
        cd "${basedir}/amzn/${ver}/${arch}" || exiterr "no ${amazonver} dir found"

        wget "${repodataurl}/repodata/primary.sqlite.${archive}"

        $archiver -d "primary.sqlite.${archive}"
        rm -f "primary.sqlite.${archive}"

        packages=$(sqlite3 primary.sqlite "select location_href FROM packages WHERE name like 'kernel-debuginfo%' and name not like '%common%'" | sed 's#\.\./##g')
        rm -f primary.sqlite

        for line in $packages; do
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

            curl -4 "${repofilesurl}/${url}" -o ${version}.rpm
            if [ ! -f "${version}.rpm" ]; then
                warn "${version}.rpm could not be downloaded"
                continue
            fi

            vmlinux=.$(rpmquery -qlp "${version}.rpm" 2>&1 | grep vmlinux)
            echo "INFO: extracting vmlinux from: ${version}.rpm"
            rpm2cpio "${version}.rpm" | cpio --to-stdout -i "${vmlinux}" > "./${version}.vmlinux" || \
            {
                warn "could not deal with ${version}, cleaning and moving on..."
                rm -rf "${basedir}/amzn/${ver}/${arch}/usr"
                rm -rf "${version}.rpm"
                rm -rf "${version}.vmlinux"
                touch "${version}.failed"
                continue
            }

            # generate BTF raw file from DWARF data
            echo "INFO: generating BTF file: ${version}.btf"
            pahole --btf_encode_detached "${version}.btf" "${version}.vmlinux"
            tar cvfJ "./${version}.btf.tar.xz" "${version}.btf"

            rm "${version}.rpm"
            rm "${version}.btf"
            rm "${version}.vmlinux"

        done

        rm -f packages
        cd "${origdir}" >/dev/null || exit
    done #amazonver

done #arch


###
### 5. Debian (stretch, buster, bullseye)
###

regex="linux-image-[0-9]+\.[0-9]+\.[0-9].*-dbg"
for arch in x86_64 arm64; do

    for debianver in stretch buster bullseye; do
        if [ "${1}" != "${debianver}" ]; then
            continue
        fi

        case "${debianver}" in
            "stretch")
                debian_number=9
            ;;
            "buster")
                debian_number=10
            ;;
            "bullseye")
                debian_number=11
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
        repository="http://ftp.debian.org/debian"

        mkdir -p "${basedir}/debian/${debian_number}/${arch}"
        cd "${basedir}/debian/${debian_number}/${arch}" || exiterr "no ${debian_number} dir found"

        wget ${repository}/dists/${debianver}/main/binary-${altarch}/Packages.gz -O ${debianver}.gz
        if [ ${debian_number} -lt 11 ]; then
            wget ${repository}/dists/${debianver}-updates/main/binary-${altarch}/Packages.gz -O ${debianver}-updates.gz
        fi

        [ ! -f ${debianver}.gz ] && exiterr "no ${debianver}.gz packages file found"
        if [ ${debian_number} -lt 11 ]; then
            [ ! -f ${debianver}-updates.gz ] && exiterr "no ${debianver}-updates.gz packages file found"
        fi

        gzip -d ${debianver}.gz
        grep -E '^(Package|Filename):' ${debianver} | grep --no-group-separator -A1 -E "^Package: ${regex}" > packages
        if [ ${debian_number} -lt 11 ]; then
            gzip -d ${debianver}-updates.gz
            grep -E '^(Package|Filename):' ${debianver}-updates | grep --no-group-separator -A1 -E "Package: ${regex}" >> packages
        fi
        rm -f ${debianver} ${debianver}-updates

        grep "Package:" packages | sed 's:Package\: ::g' | sort | while read -r package; do

            filepath=$(grep -A1 "${package}" packages | grep -v "^Package: " | sed 's:Filename\: ::g')
            url="${repository}/${filepath}"
            filename=$(basename "${filepath}")
            version=$(echo "${filename}" | sed 's:linux-image-::g' | sed 's:-dbg.*::g' | sed 's:unsigned-::g')

            echo URL: "${url}"
            echo FILEPATH: "${filepath}"
            echo FILENAME: "${filename}"
            echo VERSION: "${version}"

            if [ -f "${version}.btf.tar.xz" ] || [ -f "${version}.failed" ]; then
                info "file ${version}.btf already exists"
                continue
            fi

            if [ ! -f "${version}.ddeb" ]; then
                curl -4 "${url}" -o ${version}.ddeb
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
                rm -rf "${basedir}/debian/${debian_number}/${arch}/usr"
                rm -rf "${version}.ddeb"
                touch "${version}.failed"
                bash
                continue
            }

            mv "./usr/lib/debug/boot/vmlinux-${version}" "./${version}.vmlinux" || \
            {
                warn "could not rename vmlinux ${version}, cleaning and moving on..."
                rm -rf "${basedir}/debian/${debian_number}/${arch}/usr"
                rm -rf "${version}.ddeb"
                touch "${version}.failed"
                continue

            }

            rm -rf "./usr/lib/debug/boot"

            pahole --btf_encode_detached "${version}.btf" "${version}.vmlinux"
            tar cvfJ "./${version}.btf.tar.xz" "${version}.btf"

            rm "${version}.ddeb"
            rm "${version}.btf"
            rm "${version}.vmlinux"

        done

        rm -f packages
        cd "${origdir}" >/dev/null || exit

    done

done # arch

###
### 6. ORACLE LINUX (ol7)
###

for arch in x86_64 arm64; do

    for olver in ol7; do

        if [ "${1}" != "${olver}" ]; then
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

        regex="kernel(-uek)?-debuginfo-[0-9].*${altarch}.rpm"

        olrel=$1
        origdir=$(pwd)

        case "${olver}" in
            "ol7")
                ver="7"
                repository="https://oss.oracle.com/ol7/debuginfo/"
            ;;
        esac

        mkdir -p "${basedir}/ol/${ver}/${arch}"
        cd "${basedir}/ol/${ver}/${arch}" || exiterr "no ${ver} dir found"

        info "downloading ${repository} information"
        lynx -dump -listonly ${repository} | tail -n+4 > "${olrel}"
        [[ ! -f ${olrel} ]] && exiterr "no ${olrel} packages file found"
        grep -E "${regex}" "${olrel}" | awk '{print $2}' >temp
        mv temp packages
        rm "${olrel}"

        sort packages | while read -r line; do

            url=${line}
            filename=$(basename "${line}")
            # shellcheck disable=SC2001
            version=$(echo "${filename}" | sed -r 's:(kernel(-uek)?-debuginfo-)(.*).rpm:\3:g')
            shortversion=$(echo "${version}" | sed -r 's:([0-9]+(\.[0-9]+){2}-[0-9]+(\.[0-9]+){0,3}).*:\1:g')

            # "max" versions have the right-most decimal decremented by 1 because the version check is inclusive,
            # this will prevent inclusion of those versions because they already provide BTFs
            case $version in
                *"uek"*)
                    kernelmin="4.14.35-1902.300.11" #uek begin eBPF support
                    kernelmax="5.4.17-2136.301.1.1" #uek begin BTF support version -1
                ;;
                *)
                    kernelmin="3.10.0-1127" #ol kernel begin eBPF support
                    kernelmax="3.10.0-1160.49.0" #ol kernel begin BTF support version -1
                ;;
            esac

            sorted_vers="$(printf '%s\n' "$kernelmin" "$shortversion" "$kernelmax" | sort -V)"
            if ! [[ "$(echo "$sorted_vers" | head -n1)" = "$kernelmin" && "$(echo "$sorted_vers" | tail -n1)" = "$kernelmax" ]]; then
                #kernel doesn't support eBPF or already provides BTFs
                continue
            fi

            echo URL: "${url}"
            echo FILENAME: "${filename}"
            echo VERSION: "${version}"

            if [ -f "${version}.btf.tar.xz" ] || [ -f "${version}.failed" ]; then
                info "file ${version}.btf already exists"
                continue
            fi

            curl -4 "${url}" -o ${version}.rpm
            if [ ! -f "${version}.rpm" ]; then
                warn "${version}.rpm could not be downloaded"
                continue
            fi

            vmlinux=.$(rpmquery -qlp "${version}.rpm" 2>&1 | grep vmlinux)
            info "extracting vmlinux from: ${version}.rpm"
            rpm2cpio "${version}.rpm" | cpio --to-stdout -i "${vmlinux}" > "./${version}.vmlinux" || \
            {
                warn "could not deal with ${version}, cleaning and moving on..."
                rm -rf "${basedir}/ol/${ver}/${arch}/usr"
                rm -rf "${version}.rpm"
                rm -rf "${version}.vmlinux"
                touch "${version}.failed"
                continue
            }

            # generate BTF raw file from DWARF data
            info "generating BTF file: ${version}.btf"
            pahole --btf_encode_detached="${version}.btf" "${version}.vmlinux"
            # pahole "${version}.btf" > "${version}.txt"
            tar cvfJ "./${version}.btf.tar.xz" "${version}.btf"

            rm "${version}.rpm"
            rm "${version}.btf"
            # rm "${version}.txt"
            rm "${version}.vmlinux"

        done

        rm -f packages
        cd "${origdir}" >/dev/null || exit

    done #olver

done #arch

exit 0