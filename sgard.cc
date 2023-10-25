///
/// \file sgard.cc
/// \author K. Isom <kyle@imap.cc>
/// \date 2023-10-17
/// \brief Shimmering Clarity Gardener: dot-file management.
///

#include <iostream>

#include <scsl/Flags.h>


int
main(int argc, char *argv[])
{
	auto flag = new scsl::Flags("sgard", "Shimmering Clarity Gardener: dot-file management");

	auto parseResult = flag->Parse(argc, argv);
	if (parseResult != scsl::Flags::ParseStatus::OK) {
		std::cerr << "failed to parse flags: "
			  << scsl::Flags::ParseStatusToString(parseResult)
			  << "\n";
		flag->Usage(std::cerr, 1);
	}
}
