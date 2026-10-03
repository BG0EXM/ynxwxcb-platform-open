"""生成思源宋体子集所需字符表：GB2312 一级常用字 3755 个 + 前端源码中实际出现的全部汉字 + 常用标点/数字/字母。"""
import pathlib, sys

src_dir = pathlib.Path(sys.argv[1])
out = pathlib.Path(sys.argv[2])

chars = set()
# GB2312 一级汉字：区 16-55
for hi in range(0xB0, 0xD8):
    for lo in range(0xA1, 0xFF):
        try:
            chars.add(bytes([hi, lo]).decode('gb2312'))
        except UnicodeDecodeError:
            pass
level1 = len(chars)

# 源码中实际用到的字（涵盖二级字、专有名词，如“伊宁”等）
for p in src_dir.rglob('*'):
    if p.suffix in ('.vue', '.js', '.css', '.html') and p.is_file():
        for ch in p.read_text(encoding='utf-8', errors='ignore'):
            if '\u4e00' <= ch <= '\u9fff' or '\u3000' <= ch <= '\u303f' or '\uff00' <= ch <= '\uffef':
                chars.add(ch)

# ASCII 可见字符 + 常用中文标点
chars.update(chr(c) for c in range(0x20, 0x7f))
chars.update('，。、；：？！“”‘’（）《》〈〉【】—…·〔〕「」『』～％＋－×÷＝')

out.write_text(''.join(sorted(chars)), encoding='utf-8')
print(f'GB2312 一级: {level1}, 合计: {len(chars)}')
