cmake_minimum_required(VERSION 3.22)

project(sgard VERSION 0.0.1
	LANGUAGES CXX
	DESCRIPTION "Shimmer Clarity Gardener: A dot-files manager.")

set(CMAKE_CXX_STANDARD 17)
set(CMAKE_VERBOSE_MAKEFILES TRUE)

find_package(Protobuf REQUIRED)
find_package(scsl REQUIRED)

file(GLOB ProtoFiles "${CMAKE_CURRENT_SOURCE_DIR}/proto/*.proto")
PROTOBUF_GENERATE_CPP(ProtoSources ProtoHeaders ${ProtoFiles})
include_directories(${CMAKE_CURRENT_BINARY_DIR})
include_directories(${SCSL_INCLUDE_DIRS})

add_library(protolib STATIC ${ProtoSources} ${ProtoHeaders})
target_link_libraries(protolib INTERFACE ${Protobuf_LIBRARIES})

add_compile_options(
	"-static"
	"-Werror" "-Wextra" "-Wall"
	"-Weffc++"
)

if (CMAKE_BUILD_TYPE STREQUAL Debug)
	add_compile_options("-g" "-fsanitize=address")
endif ()

set(HEADER_FILES

)

set(SOURCE_FILES sgard.cc)

add_executable(sgard ${SOURCE_FILES})
target_link_libraries(sgard protolib ${SCSL_LIBRARIES})
