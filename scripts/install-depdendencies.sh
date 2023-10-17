#!/usr/bin/env bash

#####################################################################
# This script attempts to install the appopriate build dependencies #
# for the host system.                                              #
#                                                                   #
# For platforms marked as unverified, it means that I was able to   #
# start a Docker container for that platform and could look up the  #
# right package names. I haven't actually tried building on these   #
# platforms.                                                        #
#                                                                   #
# This is primarily developed on the latest Ubuntu LTS release and  #
# MacOS; other platforms are best-effort.                           #
#####################################################################

set -eu

install_debianesque () {
	echo "[+] distribution is ${DISTRIB_ID}, choosing Debianesque install."

	echo "[+] installing tools"
	sudo apt-get install git cmake clang scdoc

	echo "[+] installing protobuf libraries and development headers"
	sudo apt-get install protobuf-compiler libprotobuf-dev

	echo "[+] installing gRPC libraries and development headers"
	sudo apt-get install protobuf-compiler-grpc grpc-proto libgrpc++-dev
}

install_redhat () {
	echo "[+] distribution is ${DISTRIB_ID}, choosing Redhat install."
	echo "[!] WARNING: installation for Redhat systems is unverified."

	echo "[+] installing tools"
	sudo dnf install git cmake clang scdoc

	echo "[+] installing libraries and development headers"
	sudo dnf install protobuf-compiler libprotobuf-dev
}

install_alpine () {
	echo "[+] distribution is ${DISTRIB_ID}, choosing Alpine install."
	echo "[!] WARNING: installation for Alpine systems is unverified."

	echo "[+] installing tools"
	sudo dnf install git cmake clang scdoc

	echo "[+] installing libraries and development headers"
	sudo dnf install sdl2-dev freetype-dev protobuf-compiler-grpc
}

install_macos () {
	# TODO: consider supporting macports?
	echo "[+] host system is MacOS"

	echo "[+] installing tools"
	brew install git cmake scdoc

	echo "[+] installing libraries and development headers"
	# TODO: look up proper package names in homebrew
}


install_linux () {
	echo "[+] host system is Linux"
	[[ -f "/etc/lsb-release" ]] && source /etc/lsb-release
	if [ -z "${DISTRIB_ID}" ]
	then
		if [ -d /etc/apt ]
		then
			DISTRIB_ID="apt-based"
		elif [ -f /etc/alpine-release ]
		then
			DISTRIB_ID=Alpine
		elif [ -d /etc/dnf -o /etc/yum.repos.d ]
		then
			# I don't use Fedora, this is based on a cursory
			# glance at the filesystem on a Docker image.
			DISTRIB_ID="Fedora"
		fi
	fi

	case ${DISTRIB_ID} in
		Ubuntu)		install_debianesque ;;
		Debian)		install_debianesque ;;
		apt-based)	install_debianesque ;;
		Fedora)		install_redhat ;;
		Alpine)		install_alpine ;;

		*)
			echo "[!] distribution ${DISTRIB_ID} isn't supported in this script." > /dev/null
			;;
	esac
}


case "$(uname -s)" in
	Linux)		install_linux ;;
	Darwin)		install_macos ;;
	*)
		echo "[!] platform $(uname -s) isn't supported in this script." > /dev/null
		;;
esac


