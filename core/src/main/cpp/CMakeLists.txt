cmake_minimum_required(VERSION 3.0)

# A pinned detached submodule need not have any remote branch pointing to it.
set(MIHOMO_SOURCE "${CMAKE_CURRENT_SOURCE_DIR}/../../foss/golang/clash")
execute_process(
  COMMAND git rev-parse --short=7 HEAD
  WORKING_DIRECTORY "${MIHOMO_SOURCE}"
  OUTPUT_VARIABLE COMMIT_HASH
  OUTPUT_STRIP_TRAILING_WHITESPACE
  RESULT_VARIABLE GIT_RESULT
)
if (NOT GIT_RESULT EQUAL 0 OR NOT COMMIT_HASH MATCHES "^[0-9a-f]+$")
  message(FATAL_ERROR "Cannot determine pinned Mihomo revision")
endif ()
execute_process(
  COMMAND git show -s --format=%cs HEAD
  WORKING_DIRECTORY "${MIHOMO_SOURCE}"
  OUTPUT_VARIABLE SOURCE_DATE
  OUTPUT_STRIP_TRAILING_WHITESPACE
  RESULT_VARIABLE DATE_RESULT
)
if (NOT DATE_RESULT EQUAL 0 OR NOT SOURCE_DATE MATCHES "^[0-9]+-[0-9]+-[0-9]+$")
  message(FATAL_ERROR "Cannot determine pinned Mihomo source date")
endif ()
string(REPLACE "-" "" SOURCE_DATE "${SOURCE_DATE}")
string(SUBSTRING "${SOURCE_DATE}" 2 6 COMPILE_TIME)
set(GIT_VERSION "riko_${COMMIT_HASH}_${COMPILE_TIME}")
message(STATUS "Riko core source version = ${GIT_VERSION}")

# 保存变量到文件
configure_file(${CMAKE_CURRENT_SOURCE_DIR}/version.h.in ${CMAKE_CURRENT_SOURCE_DIR}/version.h @ONLY)

project(clash-bridge C)


set(CMAKE_POSITION_INDEPENDENT_CODE on)
set(CMAKE_C_FLAGS_RELEASE "${CMAKE_C_FLAGS_RELEASE} -O3")

set(GO_OUTPUT_BASE ${GO_OUTPUT}/${FLAVOR_NAME})

if ("${CMAKE_BUILD_TYPE}" STREQUAL "Debug")
    set(GO_OUTPUT_BASE "${GO_OUTPUT_BASE}Debug")
elseif ("${CMAKE_BUILD_TYPE}" STREQUAL "Release")
    set(GO_OUTPUT_BASE "${GO_OUTPUT_BASE}Release")
elseif ("${CMAKE_BUILD_TYPE}" STREQUAL "RelWithDebInfo")
    set(GO_OUTPUT_BASE "${GO_OUTPUT_BASE}Release")
else ()
    message(FATAL_ERROR "Unknown build type ${CMAKE_BUILD_TYPE}")
endif ()

include_directories("${GO_OUTPUT_BASE}/${CMAKE_ANDROID_ARCH_ABI}")
include_directories("${GO_SOURCE}")

link_directories("${GO_OUTPUT_BASE}/${CMAKE_ANDROID_ARCH_ABI}")

add_library(bridge SHARED main.c jni_helper.c bridge_helper.c)
target_link_libraries(bridge log clash)
