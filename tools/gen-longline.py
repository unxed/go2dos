#!/usr/bin/env python3
"""Generate longline.com test program with 160-character output."""

# COM file format: starts at 0x100 (256)
# We'll generate a simple program that prints 160 A's and exits

# Code section
code = bytearray()
code += bytes([0xBA])  # mov dx, ...
code += bytes([0x0C, 0x01])  # address of msg (0x010C = 268)
code += bytes([0xB4, 0x09])  # mov ah, 09h
code += bytes([0xCD, 0x21])  # int 21h
code += bytes([0xB8, 0x00, 0x4C])  # mov ax, 4C00h
code += bytes([0xCD, 0x21])  # int 21h

# Pad to 0x0C (12 bytes of code)
assert len(code) == 12

# Message section (starts at offset 0x0C = 12)
msg = b'A' * 160 + b'$'

# Build the complete COM file
com_data = code + msg

# Write the file
with open('testdata/progs/longline.com', 'wb') as f:
    f.write(com_data)

print(f"Generated longline.com: {len(com_data)} bytes")
