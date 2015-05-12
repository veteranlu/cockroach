#!/bin/bash

set -e

outdir="${TMPDIR}"
if [ "${CIRCLE_ARTIFACTS}" != "" ]; then
    outdir="${CIRCLE_ARTIFACTS}"
fi

builder=$(dirname $0)/builder.sh

# # 1. Run "make check" to verify coding guidelines.
# time ${builder} make GITHOOKS= check | tee "${outdir}/check.log"; test ${PIPESTATUS[0]} -eq 0

# # 2. Verify that "go generate" was run.
# time ${builder} /bin/bash -c "(go generate ./... && git ls-files --modified --deleted --others --exclude-standard | diff /dev/null -) || (git add -A && git diff -u HEAD && false)" | tee "${outdir}/generate.log"; test ${PIPESTATUS[0]} -eq 0

# # 3. Run "make testrace".
# match='^panic|^[Gg]oroutine \d+|(read|write) by.*goroutine|DATA RACE'
# time ${builder} make GITHOOKS= testrace \
#      RACETIMEOUT=5m TESTFLAGS='-v' | \
#     tee "${outdir}/testrace.log" | \
#     grep -E "^\--- (PASS|FAIL)|^(FAIL|ok)|${match}"

# # 3a. Translate the log output to xml to integrate with CircleCI
# # better.
# if [ "${CIRCLE_TEST_REPORTS}" != "" ]; then
#     if [ -f "${outdir}/testrace.log" ]; then
#         mkdir -p "${CIRCLE_TEST_REPORTS}/race"
# 	${builder} go2xunit < "${outdir}/testrace.log" \
# 	       > "${CIRCLE_TEST_REPORTS}/race/testrace.xml"
#     fi
# fi

# # 3b. Generate the excerpt output and fail if it is non-empty.
# find "${outdir}" -name '*.log' -type f -exec \
#      grep -B 5 -A 10 -E "^\-{0,3} *FAIL|${match}" {} ';' > "${outdir}/excerpt.txt"
# if [ -s "${outdir}/excerpt.txt" ]; then
#     echo "FAIL: excerpt.txt is not empty"
#     exit 1
# fi

# 4. Run the acceptance tests.
set -x
gopath0="${GOPATH%%:*}"
${builder} echo hello
if [ "${PWD}" != "${gopath0}/src/github.com/cockroachdb/cockroach" ]; then
    mkdir -p "${gopath0}/src/github.com/cockroachdb"
    rmdir "${gopath0}/src/github.com/cockroachdb/cockroach"
    ln -sf "${PWD}" "${gopath0}/src/github.com/cockroachdb/cockroach" >& /dev/null || true
    cd "${gopath0}/src/github.com/cockroachdb/cockroach"
fi
${builder} echo world
time $(dirname $0)/../acceptance/run.sh
