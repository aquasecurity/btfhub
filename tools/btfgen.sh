#!/bin/bash

usage() {
    echo "Usage: $0 [-a <x86_64|arm64> -o <file01.bpf.o> -o <file02.bpf.o>] [-j <num_jobs>]" 1>&2
    exit 1
}

# Set default jobs to 1
j=1

# Get number of CPUs for validation when user sets -j
if command -v nproc > /dev/null 2>&1; then
    max_jobs=$(nproc)
elif [ -f /proc/cpuinfo ]; then
    max_jobs=$(grep -c ^processor /proc/cpuinfo)
else
    max_jobs=1
fi
# Fallback to 1 if detection failed or result is empty/non-numeric
if ! [ "${max_jobs}" -ge 1 ] 2> /dev/null; then
    max_jobs=1
fi

o=()

while getopts ":a:o:j:" opt; do
    case "${opt}" in
        a)
            a="${OPTARG}"
            [[ "${a}" != "x86_64" && "${a}" != "arm64" ]] && usage
            ;;
        o)
            [[ ! -f "${OPTARG}" ]] && {
                echo "error: could not find bpf object: ${OPTARG}"
                usage
            }
            o+=("${OPTARG}")
            ;;
        j)
            j="${OPTARG}"
            # Validate it's a positive integer
            if ! [ "${j}" -ge 1 ] 2> /dev/null; then
                echo "error: -j must be a positive integer (got: ${j})"
                usage
            fi
            # Cap at CPU count if exceeded
            if [ "${j}" -gt "${max_jobs}" ]; then
                echo "warning: -j ${j} exceeds CPU count (${max_jobs}), using ${max_jobs}"
                j="${max_jobs}"
            fi
            ;;
        *)
            usage
            ;;
    esac
done
shift $((OPTIND - 1))

if [ -z "${a}" ] || [ ${#o[@]} -eq 0 ]; then
    usage
fi

obj_cmdline=""
for ofile in "${o[@]}"; do
    obj_cmdline+="${ofile} "
done

basedir=$(dirname "${0}")/..
if [ "${basedir}" == "." ]; then
    basedir="$(pwd)/.."
fi

if [ ! -d "${basedir}/archive" ]; then
    echo "error: could not find archive directory"
    exit 1
fi

cd "${basedir}" || exit 1

btfgen="$(which bpftool)"
if [ -z "${btfgen}" ]; then
    btfgen=/usr/sbin/bpftool
fi

if [ ! -x "${btfgen}" ]; then
    echo "error: could not find bpftool (w/ btfgen patch) tool"
    exit 1
fi

# Track background jobs for cleanup
declare -a background_pids=()

function ctrlc() {
    echo "Exiting due to ctrl-c..."

    # Kill all background jobs
    for pid in "${background_pids[@]}"; do
        if kill -0 "${pid}" 2> /dev/null; then
            kill "${pid}" 2> /dev/null
        fi
    done

    # Wait a bit for graceful shutdown
    sleep 1

    # Force kill if still running
    for pid in "${background_pids[@]}"; do
        if kill -0 "${pid}" 2> /dev/null; then
            kill -9 "${pid}" 2> /dev/null
        fi
    done

    exit 2
}

trap ctrlc SIGINT
trap ctrlc SIGTERM

# Jobs extract and minimize inside their own directory under work_dir, and
# record there which sources they minimized (sources/<sha256>/).
work_dir="$(mktemp -d)" || exit 1
mkdir "${work_dir}/sources"
trap 'rm -rf "${work_dir}"' EXIT

# clean custom-archive directory
find ./custom-archive -mindepth 1 -maxdepth 1 -type d -exec rm -rf {} \;

# Function to process a single BTF file
process_btf_file() {
    local file="$1"
    local btfgen="$2"
    local obj_cmdline="$3"
    local work_dir="$4"

    local dir
    local out_dir
    local extracted
    local temp_dir
    local hash
    local source_dir

    dir="$(dirname "${file}")"
    dir=${dir/\.\/archive\//}
    out_dir="./custom-archive/${dir}"

    # Extract into this job's own directory: kernels with the same file name
    # (e.g. one kernel in two Ubuntu releases) must not share a path
    temp_dir="$(mktemp -d -p "${work_dir}")" || return 1
    extracted="$(tar -C "${temp_dir}" -xvJf "${file}" 2> /dev/null)"
    local ret=$?

    if [[ ${ret} -ne 0 || ! -f "${temp_dir}/${extracted}" ]]; then
        printf "[FAIL] %s (extraction failed)\n" "$(basename "${file}")"
        rm -rf "${temp_dir}"
        return 1
    fi

    mkdir -p "${out_dir}"

    # Many kernels share the same BTF (the same kernel in two releases, or
    # releases that did not change any type). Minimize each distinct BTF
    # once: the first job to create its source directory minimizes it, and
    # the others are listed there, to get a copy once all jobs are done.
    hash="$(sha256sum "${temp_dir}/${extracted}" | cut -d' ' -f1)"
    source_dir="${work_dir}/sources/${hash}"
    if ! mkdir "${source_dir}" 2> /dev/null; then
        printf '%s\n' "${out_dir}/${extracted}" >> "${source_dir}/copies"
        rm -rf "${temp_dir}"
        printf "[SAME] %s\n" "${extracted}"
        return 0
    fi

    # Generate minimized BTF file
    # shellcheck disable=SC2086
    "${btfgen}" gen min_core_btf "${temp_dir}/${extracted}" "${out_dir}/${extracted}" ${obj_cmdline}
    local btfgen_ret=$?

    rm -rf "${temp_dir}"

    if [[ ${btfgen_ret} -eq 0 ]]; then
        printf '%s\n' "${out_dir}/${extracted}" > "${source_dir}/output"
        printf "[SUCCESS] %s\n" "${extracted}"
        return 0
    else
        printf "[FAIL] %s\n" "${extracted}"
        return 1
    fi
}

# Export the function so it can be used by background processes
export -f process_btf_file

echo "Using ${j} parallel jobs for BTF processing..."

# Ensure output is line-buffered for better real-time display
stty -icanon min 1 time 0 2> /dev/null || true

# Initialize job control variables
job_count=0
failed_jobs=0
completed_jobs=0
start_time="$(date +%s)"

# Collect all BTF files to process
btf_files=()
for dir in $(find ./archive/ -iregex ".*${a}.*" -type d | sed 's:\.\/archive\/::g' | sort -u); do
    while IFS= read -r -d '' file; do
        btf_files+=("${file}")
    done < <(find "./archive/${dir}" -name "*.tar.xz" -print0)
done

total_files=${#btf_files[@]}
echo "Found ${total_files} BTF files to process"

if [[ ${total_files} -eq 0 ]]; then
    echo "No BTF files found for architecture ${a}"
    exit 0
fi

# Show system info
echo "System: $(nproc) CPU cores, $(free -h | awk '/^Mem:/ {print $2}') RAM"
echo "Started at: $(date)"
echo

# Process files in parallel with job control
for file in "${btf_files[@]}"; do
    # Wait if we've reached the maximum number of parallel jobs
    while [[ ${job_count} -ge ${j} ]]; do
        # Wait for any background job to complete
        wait -n
        exit_code=$?
        ((job_count--))
        ((completed_jobs++))

        # Track failed jobs
        if [[ ${exit_code} -ne 0 ]]; then
            ((failed_jobs++))
        fi
    done

    # Start new background job
    process_btf_file "${file}" "${btfgen}" "${obj_cmdline}" "${work_dir}" &
    pid=$!
    background_pids+=("${pid}")
    ((job_count++))
done

# Wait for all remaining background jobs to complete
echo -e "\nWaiting for remaining ${job_count} jobs to complete..."
while [[ ${job_count} -gt 0 ]]; do
    wait -n
    exit_code=$?
    ((job_count--))
    ((completed_jobs++))

    if [[ ${exit_code} -ne 0 ]]; then
        ((failed_jobs++))
    fi
done

# Copy each minimized BTF to the kernels that share its source
copied_files=0
for source_dir in "${work_dir}"/sources/*; do
    [[ -f "${source_dir}/copies" ]] || continue
    output=""
    [[ -f "${source_dir}/output" ]] && output="$(< "${source_dir}/output")"
    while IFS= read -r copy; do
        if [[ -n "${output}" ]] && cp "${output}" "${copy}"; then
            ((copied_files++))
        else
            printf "[FAIL] %s (same BTF as a kernel that failed)\n" "$(basename "${copy}")"
            ((failed_jobs++))
        fi
    done < "${source_dir}/copies"
done

# Final summary with timing
end_time="$(date +%s)"
total_elapsed=$((end_time - start_time))
average_rate=$((total_files * 60 / (total_elapsed + 1)))

echo -e "\n\n🎉 BTF processing completed!"
echo "📊 Total files: ${total_files}"
echo "✅ Completed: ${completed_jobs} (${copied_files} copied from a kernel with the same BTF)"
echo "❌ Failed jobs: ${failed_jobs}"
echo "⚙️ Parallel jobs: ${j}"
echo "⏱️ Total time: $((total_elapsed / 60))m $((total_elapsed % 60))s"
echo "🚀 Average rate: ${average_rate} files/min"

if [[ ${failed_jobs} -gt 0 ]]; then
    echo "error: Some BTF files failed to process"
    exit 1
fi
