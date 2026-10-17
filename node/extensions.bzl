"""Node repositories: the kernel build tools, mkfs.erofs, and QEMU.

Each package is a sha256 pin in this file. Alpine packages are musl builds.
A wrapper starts each one with the musl loader from the same packages, so
the host libc is not an input. The QEMU pins match efi-boot-shim.

The kernel source and the kernel LLVM are in tools/extensions.bzl, pinned in
internal/sums/checksums.sha256 like every other archive.
"""

load("//node:alpine.bzl", "alpine_packages")
load("//node:brew.bzl", "brew_bottles")

_MIRROR = "https://dl-cdn.alpinelinux.org/alpine/v3.24"

# flex, bison, m4, bc, make, xz, zstd, mkfs.erofs, and static libelf for objtool.
_HOSTTOOLS_X86_64 = {
    "main/musl-1.2.6-r2.apk": "573712e2f49c15bfc20a2699f204acdfc74c772722b15e7353d768057fae0e71",
    "main/flex-2.6.4-r8.apk": "18694dcfa86e5ffe689e81be088ec2b7d6c632d76f645e860cec9ad8eaa32225",
    "main/bison-3.8.2-r3.apk": "07c588c6502ece84cf6acb0ec3c6c3b592948de1e4b5590c63a17c9ec0b694f7",
    "main/m4-1.4.20-r1.apk": "e163b45e0fae7b50f4abec37833a729453cea38a5d74c9ed1bdb04a475eeba70",
    "main/bc-1.08.2-r1.apk": "3f5cb1c1af65d669d52773f918b7dc75bee7af2d75993cd64045ea6865b314c1",
    "main/readline-8.3.3-r1.apk": "766911ecb986a6c5cf0841a6b556cd1e9dbbf22b3559726de32416486cd15c81",
    "main/libncursesw-6.6_p20260516-r0.apk": "cf8caa8a88bc4ce9d9e395567fefaf9ef7fbd55c50ae6155b35c1b58f3755023",
    "main/ncurses-terminfo-base-6.6_p20260516-r0.apk": "ce83be6d0bd10584a53c3a5868cc1595ce5f0f8e0b621f8f7f70b6144e521fc9",
    "main/make-4.4.1-r4.apk": "a916626a876f64c6b6fb32a6bf0fa93351d09fae0aa7684135642390da647e60",
    "main/xz-5.8.4-r0.apk": "547866cbe6f4a93efc9cee3bba03629d37f5590446677bb800cf4eada4b22c7c",
    "main/xz-libs-5.8.4-r0.apk": "ab362dd515ead04c31a6c302163248da7aa4813e283c971e6dd3f789b453a1d9",
    "main/xz-dev-5.8.4-r0.apk": "9292f2044e1f93fff3997b449f95567fcd9356b931cdac1baa2578f24592732a",
    "main/xz-static-5.8.4-r0.apk": "da87145dfef2ef86c35d63e1a28cf5920e0b5c26f2e7245ecc7b62c3f1f18aba",
    "main/zstd-1.5.7-r2.apk": "774da060da278110a2075de01ecee04fb036033ec19de9f3c36f0283c150a487",
    "main/zstd-libs-1.5.7-r2.apk": "23c6065b0049b2406441564bcf0032515a43f78e80d76fcb85535a3803ef5d4e",
    "main/zstd-dev-1.5.7-r2.apk": "1b548f46268e658cdd79fc65eb3e58e627d57d8694c3615f6e9490e6f70ba0cd",
    "main/zstd-static-1.5.7-r2.apk": "d8a3eea451aa7f1355d563eddf8de31b14519ffbf3397c9365346a22ff4124d8",
    "main/elfutils-dev-0.195-r0.apk": "5695ea1ff0782068a7f16712d3665acb1226892414972a4e30dc915b38f66379",
    "main/libelf-0.195-r0.apk": "78c5a1993342ab1bdcdce30639fd0937e8000f6d0d85cea171f573d31decd036",
    "main/zlib-1.3.2-r1.apk": "63aeea03c15a2f9018f81805cfc8aa926bdf5cd68921f22149c2fbb5d0ee9f47",
    "main/zlib-dev-1.3.2-r1.apk": "50e47b778d894841a9c8ace7640edc1e585269963f29d6e4fdc008c8d6364890",
    "main/zlib-static-1.3.2-r1.apk": "1b7ce8d0515576599450144a5652eeba0e04e4b7342bba32cbc5a2a4ae256939",
    "main/lz4-libs-1.10.0-r1.apk": "3ad4912ab8ecf5f6236fdb751f88243006b3a8f152684e455b781e16284dd298",
    "main/libuuid-2.42.3-r1.apk": "8306e5bb577696c9069fe1dfd9e1dcc39d2d481c6a1b0e707fd03c3e21aa6aa2",
    "community/erofs-utils-1.9.1-r0.apk": "abe0b6153fc472c7ef4e24f23cbd7142c2cfa13d5f2ba9c4c2e12bc2c0cb52c6",
}

_HOSTTOOLS_BUILD = """
filegroup(
    name = "all",
    srcs = glob(["lib/**", "usr/**"]),
    visibility = ["//visibility:public"],
)

exports_files(["lib/ld-musl-x86_64.so.1", "usr/bin/mkfs.erofs"], visibility = ["//visibility:public"])
"""

_QEMU_BUILD = """
# QEMU from Alpine (musl). The test harness starts it with lib/ld-musl-*.so.1.
filegroup(
    name = "qemu",
    srcs = glob(["lib/**", "usr/lib/**", "usr/bin/qemu-system-x86_64", "usr/share/qemu/**"]),
    visibility = ["//visibility:public"],
)

alias(name = "qemu-system-x86_64", actual = "usr/bin/qemu-system-x86_64", visibility = ["//visibility:public"])

# OVMF from the same qemu package: edk2-x86_64-code.fd has HttpDxe.
alias(name = "ovmf_code", actual = "usr/share/qemu/edk2-x86_64-code.fd", visibility = ["//visibility:public"])

alias(name = "ovmf_vars", actual = "usr/share/qemu/edk2-i386-vars.fd", visibility = ["//visibility:public"])
"""

# QEMU 11.1.2 and its libraries, Homebrew bottle tag arm64_golden_gate (macOS 27).
_QEMU_MACOS_ARM64 = {
    "qemu": "a399faaf2f8fb45d3b1f35704041a9104f7f717e9fda1b12d4deac9e919d0d33",
    "capstone": "6b31fe379681e2c0778e30b3516cdfc2cfdff6424f666463643f1507039daf0b",
    "dtc": "92ee4960b1643f3ba722d3d0cd9eb31af546215c7eaf858e76e2c69dd2c7afe8",
    "gettext": "4db034aa2ae4b1f18d86906707ece14c895198090ad028d2fb1ae4fbeb3ca6fd",
    "glib": "ac3a58f1d2c1db194a3a5dc8484187d258b1a14e191772323937d17f5fae59fd",
    "gmp": "5116a8201d8e044aef7d486da7e57f1e4dd2d8554ea6b540b44e45c0804f8f6f",
    "gnutls": "366c9b9c0a6b246af58952c4bb5ac921ec5be87bdc23c0b9d8b2ca97b1a3d996",
    "jpeg-turbo": "7bc0f7c007a73c68da8c11055e836de66c2814c1b0ca0302da9a6317199fd37a",
    "libidn2": "bacc89595e4e8d2a4e14eb71730ae96a1ca68830068a741c59fc363a355ef1bc",
    "libpng": "7e04225ac0bc048d2b12c603f678e5fa36459909812f2f1306dbd97af337cdec",
    "libslirp": "9c0233380116a46e7d292a7f0b1b3084214f399ae839806bcff0018f581da405",
    "libssh": "48110039ef444f383b5fab938dffbca01203784eafcf2a05135477cffef15751",
    "libtasn1": "68d1748171848565af7a083ca92b834ab1fe4ac7832140b1cd22fd1fcb072fd7",
    "libunistring": "57307eb89a02e09aa4de70e09391994dfc9520d9f101ec7e64f9af21dcea2420",
    "libusb": "b462926c2314b5bb7d1a6721f9a3cf7336a4166ba67024b4bbd0226de457132a",
    "lzo": "928ed0070e25fe8082e24daa3f967657be0c1c5c72c14a3ac4cd498874eeec11",
    "ncurses": "36a1f053f1be8ce8dce8a92b68395fb1483b077d313077e9c8f4b20b6f712602",
    "nettle": "42ff85a054b4a265542175dd31f4a515e0a3e7d8113cfddacc5472555a036bc8",
    "openssl@4": "cfe1f3ee8ddafdb7f19fe4d4dd600e2b84257df4fa09a169ccb7322d38483f90",
    "p11-kit": "a7c4679040141c7c8cb0861e164126871f7e0eb485df6da5ebecc3665e185ce2",
    "pcre2": "5cac44323243138490ca74a2499fd347fa836df6b5fffa789944be12110e91ff",
    "pixman": "918ff532bf70bc121376f4b51420f44fc1abd0b9c519d7f8040f6ad9d304bc02",
    "snappy": "7f20ac629eaf9a34a0e4d0e1fe60a933d42ececf3d956988fa8b24916e467c85",
    "vde": "972ce0f14fb0fe92932f45b887019b228ac4529f06d0a95eeb88ece1359d76ba",
    "zstd": "7fb4af03304cc4897326168cdcd53290311c5543150f5502a61c3e84b6d62765",
}

# QEMU 11.0.3 for x86_64 Linux hosts, Alpine v3.24.
_QEMU_LINUX_X86_64 = {
    "main/brotli-libs-1.2.0-r1.apk": "f0823de7c6d45859aeb6a7facfc5cfbf2dadb83ad4f27c794cb6ecc74e578cee",
    "community/capstone-5.0.9-r0.apk": "168f1d03ab87ba63f14bfbef720d5fd3e907efd2b242d167916bc18ad982229b",
    "main/e2fsprogs-libs-1.47.4-r0.apk": "2e067395a6981b06cfd2cd3abf0e6c6574f7ef64d02e01f75fc5323d4a337fb9",
    "main/gdbm-1.26-r0.apk": "3278ccba30b241168ca4ab93c919be805411b918fdc062f06d3684d12dc33ae3",
    "main/glib-2.88.1-r1.apk": "2f32203b97789bc9a92e57e2d9b89eb2e6951b69534ba082a683c6e49031cb5d",
    "main/gmp-6.3.0-r4.apk": "85323ef5bce8ff234736dc2d6592ea37aebdc6348a531e27430bdfe38364d256",
    "main/gnutls-3.8.13-r0.apk": "8d3f3612145db8cd7feb83607187a60f809f06cfa3c2e74d310d58a6b053d54c",
    "main/libaio-0.3.113-r2.apk": "7553b494e3725e16de6a41be7a6a1608e7b87adfc4236d8cda14ab7bbfe05416",
    "main/libblkid-2.42.3-r1.apk": "2512c2bff496ac2b8561a3c814d21d35ce10cda579b3d410f9f96da93a5aaac1",
    "main/libbpf-1.7.0-r1.apk": "0b28a1d18657a244e493b812a3f7b9bae67166abc96e0e4dceb08bdbe09e511c",
    "main/libbz2-1.0.8-r6.apk": "2737e2b23ed08d4911ee62cd0163b5f4bf0379cacf4dc35b23a00209584c98fd",
    "main/libcap-ng-0.8.5-r2.apk": "feeb65b050ec75766057ce8f3e9611eef3c03d9eeed17d31a900ab900ed2bcf2",
    "main/libcom_err-1.47.4-r0.apk": "4723d0fb8e286dc6adb54dd3cdaedcd8db54260f75ee778a5acb4c16922c0f52",
    "main/libdw-0.195-r0.apk": "e1cc3b757cf44a38f6b160cb714bf570bc28eca11cd1ba8e8b0b54eab4837ffb",
    "main/libeconf-0.8.3-r0.apk": "2a791c7957961f830cf31b04f35f814a63afcf633badc9052425ff1cf2f64380",
    "main/libelf-0.195-r0.apk": "78c5a1993342ab1bdcdce30639fd0937e8000f6d0d85cea171f573d31decd036",
    "main/libfdt-1.7.2-r1.apk": "50ce67a6c7e783cac5c492ff305cb777b0abc02f4bbe7d3cdc5de2a908ae3ebe",
    "main/libffi-3.5.2-r1.apk": "6da1a19313a40c56cac756fb11e2c4d5267831e1c22fc8323f24c943c94da6e0",
    "main/libgcc-15.2.0-r5.apk": "393dcd32629f06d7d85409c272d142d0c082772d10b87ef55ee82f47de3be637",
    "main/libidn2-2.3.8-r0.apk": "7d038ac558dde464964b033ce5126dd2eaf84cc414dc15e77c5424a321d0ebee",
    "main/libintl-1.0-r0.apk": "cb03919cba5624b5e8759d04f3b31913b02585355988b914bc31de4746ff5ef6",
    "main/libjpeg-turbo-3.1.3-r0.apk": "6299627dd0a9156a7463511e4915a6294dfb592d8b4bdf85e55265adf1b8ea04",
    "main/libmount-2.42.3-r1.apk": "e2ae003260f2c1ec13666c6e108d5e8e3d64974b0667166a02432bd3be263a7a",
    "main/libnl3-3.11.0-r0.apk": "b8c9b0a045b1615027b58618103f3dabeca69aacb80b527ac7b96ba354aba4a4",
    "main/libpng-1.6.59-r0.apk": "d16aeb0597f2bc43e0583ac9ff779388ec160c3e22568b640122e94ba55be6d7",
    "main/libsasl-2.1.28-r9.apk": "ae838d4be2c0eba0d8cdc4fe5029fcc2cb187ec5814366b9bb954e41b8dcd1d6",
    "main/libseccomp-2.6.1-r0.apk": "b22ccfe8cb072e4c8d3bc17222eeda132e270ce6258c27622acb193bf27b48e1",
    "community/libslirp-4.9.1-r0.apk": "bd073e80c5a9379a729cde09b794e655fce36d5e2159f0901aebd65c395d2170",
    "main/libstdc++-15.2.0-r5.apk": "14c987b556f5385a5db18376e788c75f37d85321b8dc1920d926ea7daac1d6f6",
    "main/libtasn1-4.21.0-r0.apk": "407f255c5af989f4e22a5e9404c4558f6fda04f14a4209a2ca1e06c3f2ba476e",
    "main/libunistring-1.4.2-r0.apk": "3aa6030e603e34d96192742f327c0509f409c6769ce2e7b1463685dede38d714",
    "main/liburing-2.14-r0.apk": "77b170b74dd0d2da3eef4a60cf75121baf6de3b308f8b3db8808d4e697602772",
    "main/libuuid-2.42.3-r1.apk": "8306e5bb577696c9069fe1dfd9e1dcc39d2d481c6a1b0e707fd03c3e21aa6aa2",
    "main/lzo-2.10-r5.apk": "a7705056c24a8f37bd22eb8cc5a1f7713de7cd06d294baaa34ba148cbc0bbd2a",
    "main/musl-1.2.6-r2.apk": "573712e2f49c15bfc20a2699f204acdfc74c772722b15e7353d768057fae0e71",
    "main/musl-fts-1.2.7-r7.apk": "10563dc485f24f7cd987e0f3c68552e2c6417b61f5c8cb6b39cbf9af46ea09a7",
    "main/nettle-3.10.2-r0.apk": "693c1a99b781cd17149916df30113ccd6bdddac130298c17b1286fd2ae438386",
    "main/numactl-2.0.19-r0.apk": "4a78670a16d9ae7377906e2084be7430197d4b4c24877d65712dd1d4597b59db",
    "main/p11-kit-0.26.2-r0.apk": "134d653aeeb4ada32746fbc3e2a7ae4201f584725de9263e4f32503658f3ddd8",
    "main/pcre2-10.49-r0.apk": "2919897914820f42a17a8d7d37df310d2dfad1c82ebe5f539cc5c765f916752a",
    "main/pixman-0.46.4-r0.apk": "378550cbe348dda9b0733014375dba87a89f0077c5f3dbeba7dce8dbcdda707e",
    "community/qemu-11.0.3-r0.apk": "89f12874767f11fc9d6cc82d203809817ada2295213ae7964e82c9ccf6b0fb1d",
    "community/qemu-edk2-i386-vars-11.0.3-r0.apk": "8dfabc2b6e9393435b1503b3ada7c566a50277c143bc3a65a9c45261a4f358cc",
    "community/qemu-system-x86_64-11.0.3-r0.apk": "9cb201b19564ac9424d1a703e6beaf07d389643b141868088b7a3e95f6367fd9",
    "main/snappy-1.2.2-r0.apk": "facf5ac4b8ae5020ac4a3851ebcd9e318a6135b1274f023fc1851fc63a4d79a0",
    "community/vde2-libs-2.3.3-r6.apk": "ba5c8459eddfb3edc028df681a3d5f0b468002342dd93fbfe09ba56b1e8394b2",
    "main/xen-libs-4.21.2-r2.apk": "b6b0dcf1f57ff049ed44f6afb7f010faeb3dff92b6897db5c9a7b480134d48f9",
    "main/xz-libs-5.8.4-r0.apk": "ab362dd515ead04c31a6c302163248da7aa4813e283c971e6dd3f789b453a1d9",
    "main/yajl-2.1.0-r9.apk": "191009556bf22fc2e052cf38e5710016ac98c7f865fa3c3ca0e304c0d2cf60d7",
    "main/zlib-1.3.2-r1.apk": "63aeea03c15a2f9018f81805cfc8aa926bdf5cd68921f22149c2fbb5d0ee9f47",
    "main/zstd-libs-1.5.7-r2.apk": "23c6065b0049b2406441564bcf0032515a43f78e80d76fcb85535a3803ef5d4e",
}

# QEMU 11.0.3 for aarch64 Linux hosts, Alpine v3.24.
_QEMU_LINUX_AARCH64 = {
    "main/brotli-libs-1.2.0-r1.apk": "e0e7a7892f28326a9f18e09607b1aea72baedda470bc4e3d2b11262b9ae5dda3",
    "community/capstone-5.0.9-r0.apk": "d9304823c7396cfee33fa1e9250995296141c63480c921f2996b76e77089f38b",
    "main/e2fsprogs-libs-1.47.4-r0.apk": "52124971ae397d599aadd79263fcd68fc9e271922ccf8185b124f35fe24c60c8",
    "main/gdbm-1.26-r0.apk": "bad50eb26533e2f58e014fbcd3c1744f2fdb79954af6e3c188ccd059251c7790",
    "main/glib-2.88.1-r1.apk": "0ae89191a79ea5d5a02515e41f29b62e5e0a0a80fb059804bbbacac2d77cce4e",
    "main/gmp-6.3.0-r4.apk": "4dab6e86b850fe1b5bc0cf2fac17efa4d7f648bea44f3d9019694cc1e1cb12b7",
    "main/gnutls-3.8.13-r0.apk": "8678c104d4c08e7e41b5fcacdfdbffa208066bcb1c3f7f8b620e0a6bd22b3e40",
    "main/libaio-0.3.113-r2.apk": "ff918b2179f6c779630923cfd5bd140a668b7231c05414eb68d69aff8fdb5845",
    "main/libblkid-2.42.3-r1.apk": "9d0f24976c575c53c12526b6e078edf34c980c153bc8e5f1c1053cbece5b8cfa",
    "main/libbpf-1.7.0-r1.apk": "69d6435f48a738bff36c2bedd5e132799918e794e9affb9c341efce95be9d97d",
    "main/libbz2-1.0.8-r6.apk": "28cb76bb6d11d2a55c3061b5fcb2ab84996412a248b2c9fe52bb710d0a2a84e7",
    "main/libcom_err-1.47.4-r0.apk": "a75b5955ff57046e33fc2842cfe466dc77d8e68305d5c86c6646b38a069490a2",
    "main/libdw-0.195-r0.apk": "ccfe821138496ebc2e78dc8ace441522583eb06844df6ea24715e7f026b7bbe0",
    "main/libeconf-0.8.3-r0.apk": "cf0899d49ded0a6891e3cdf1b37887f2ecd77293afe1d8f3cb8937763c83f183",
    "main/libelf-0.195-r0.apk": "1c108314af454a885ac0b053201db57027470c4f2309412c016897e2a2bf5d0e",
    "main/libfdt-1.7.2-r1.apk": "3fc04330eb4ac6f449ca4409f25a262a015066533af2aa12a1d7997d9d8f50c3",
    "main/libffi-3.5.2-r1.apk": "9a67280b0c7c62db658c2d4d32ed1199903bef859c32dd4245e1a7e35c89ab4b",
    "main/libgcc-15.2.0-r5.apk": "369aaa6e9d099a737bad6dd3e6c2fe7bb1547ca26d22b94ee0411228f709b403",
    "main/libidn2-2.3.8-r0.apk": "d02ffda4510652f89c5fb89cd2e7ff721afb0c74e7d75476ad60bebe7ac0ae20",
    "main/libintl-1.0-r0.apk": "d106a13e3e68103d3a8217406aae6186289baf352d0f3d10fc21938d2dd8adb1",
    "main/libjpeg-turbo-3.1.3-r0.apk": "0e82bb8d0abad143d9a84ee32152707e5a71176bd803b070b6179e85e11bf2be",
    "main/libmount-2.42.3-r1.apk": "a2a6fc6aab7a59c70ec060f5fb8c950be42d1a017934bf32e5dd6984eaab15cc",
    "main/libnl3-3.11.0-r0.apk": "b32d23e45bde6d77989362ce4ca2989ea0f85e319eca10b9d5d73cf5b65a5460",
    "main/libpng-1.6.59-r0.apk": "bccd127f8c42bc5c21c018b6b1dc977128f3bad64c3412e541c09e96e8b227a4",
    "main/libsasl-2.1.28-r9.apk": "0be9020fdb51705c6f393bad6e533bbcd70f68c148ab0b88d35ef39a4ce8959f",
    "main/libseccomp-2.6.1-r0.apk": "9a2a2a5ff3b17ca394a484a5905bceaa57c7d4d392079f07d68a03e44cdcdcd6",
    "community/libslirp-4.9.1-r0.apk": "680c0bc3c04d9c1c127b4f960f30fd2f61bd79f0a7a599361c2f3be34e09874b",
    "main/libstdc++-15.2.0-r5.apk": "2302e766d4e4926038ec166ecb85837ee884576115236ddb565e3a5fca4a11d7",
    "main/libtasn1-4.21.0-r0.apk": "c9fcd102c576a05675ebf4d61f147e5209fd4518e8627f229032240afaf7624b",
    "main/libunistring-1.4.2-r0.apk": "58abfcbce2459071c505771a572a79953d6043d72b643ed0d5285ca39cdaf85c",
    "main/liburing-2.14-r0.apk": "388e70b040b6ec26bd8d171e514ac68626b00a1871a0134c53de22ed059c2627",
    "main/libuuid-2.42.3-r1.apk": "9ce20c7ffe2ccaa7c321893c10564abbca13c3f2edb82f60a35f1f68e004f86c",
    "main/lzo-2.10-r5.apk": "a4d2dc5c6174527784244aff061c330f5c66f28d45fa8588c8076d83f0d2822c",
    "main/musl-1.2.6-r2.apk": "5e9674b7f41152fe2119093b5cb4c13eaaadb19c2d5422b2d7267913e663ee6e",
    "main/musl-fts-1.2.7-r7.apk": "49e242db9339136148f1399d2126af277a5723a23d0be5a47fa0c94f96732e73",
    "main/nettle-3.10.2-r0.apk": "573c54ecb55b157d9cd5323eb3a9bb9d87b375ee20392a6a9d16c0ba098af9e5",
    "main/numactl-2.0.19-r0.apk": "911046ec22d596820cc37427b470f80bb095bf415c0debcc2774b7c0c43be9d8",
    "main/p11-kit-0.26.2-r0.apk": "0ad7cbbc011491b604b4d8fb8aafb6ce2a271c19eb8b785dd51f1704c61c58a5",
    "main/pcre2-10.49-r0.apk": "03efd0b29f7f38deb10a9ccb54f56e19683eb7ac27eb1f347e6dbd2e59ea000d",
    "main/pixman-0.46.4-r0.apk": "022ab7d2b766f3893e6c457a03c723fe3e79e9f009d8a62d06f1692d9f2fc77f",
    "community/qemu-11.0.3-r0.apk": "3f87d65b2c350dfeaec860bce485860537d3b0e59cbe92ecad09552bb77d02ee",
    "community/qemu-edk2-i386-vars-11.0.3-r0.apk": "4d42437b9f22d8216cd6427dad45d6a4c648d16b11d1b9782fa581d2e0c20bc2",
    "community/qemu-system-x86_64-11.0.3-r0.apk": "c415f10da231382ac8f2015b03d461ae4e652aec19cf547b058aa3019eacb3e7",
    "main/snappy-1.2.2-r0.apk": "3562c65dd72f5ef604b9dad8c8fad32b422b5384f36957ff1e8f4d982882f52b",
    "community/vde2-libs-2.3.3-r6.apk": "89a996668d33310a1098bdccf94a18cf8467675f48fdfd6b285cf09ff4faa1c0",
    "main/xen-libs-4.21.2-r2.apk": "2cff7d024e6692776fd1a3667566862a11898091e67af9bfea850b65edab5a36",
    "main/xz-libs-5.8.4-r0.apk": "9be375832e747afe14f98e400e8b55d8f6fde25e41663090c81600628daab093",
    "main/yajl-2.1.0-r9.apk": "897030fa9e1724da03bf7259b756a36406cf26d60b1bb870284745052e622c97",
    "main/zlib-1.3.2-r1.apk": "1f77a542ac6634761b9fe38d8bdca19b154071dcfa25432e73f2bc8a0740a33f",
    "main/zstd-libs-1.5.7-r2.apk": "2bb5136c89f5b0bbe1554c8915a3b520d5aa63ae2a51d4d821eb81698db5a818",
}

def _node_repos_impl(_module_ctx):
    alpine_packages(
        name = "node_hosttools_linux_x86_64",
        arch = "x86_64",
        build_file_content = _HOSTTOOLS_BUILD,
        mirror = _MIRROR,
        packages = _HOSTTOOLS_X86_64,
    )
    brew_bottles(name = "node_qemu_macos_arm64", bottles = _QEMU_MACOS_ARM64)
    alpine_packages(
        name = "node_qemu_linux_x86_64",
        arch = "x86_64",
        build_file_content = _QEMU_BUILD,
        mirror = _MIRROR,
        packages = _QEMU_LINUX_X86_64,
    )
    alpine_packages(
        name = "node_qemu_linux_aarch64",
        arch = "aarch64",
        build_file_content = _QEMU_BUILD,
        mirror = _MIRROR,
        packages = _QEMU_LINUX_AARCH64,
    )

node_repos = module_extension(implementation = _node_repos_impl)
