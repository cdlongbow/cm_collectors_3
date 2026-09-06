export function parseMobileVtt(content: string) {
  const timestamp = (value: string) => value.replace(',', '.').split(':').reduce((total, part) => total * 60 + Number(part), 0);
  return content.replace(/\r\n?/g, '\n').split(/\n{2,}/).flatMap(block => {
    const lines = block.split('\n');
    const index = lines.findIndex(line => line.includes('-->'));
    if (index < 0) return [];
    const match = lines[index].match(/([\d:.]+)\s+-->\s+([\d:.]+)/);
    if (!match) return [];
    const start = timestamp(match[1]), end = timestamp(match[2]);
    if (!Number.isFinite(start) || !Number.isFinite(end) || start < 0 || end < start) return [];
    const decoder = document.createElement('textarea');
    decoder.innerHTML = lines.slice(index + 1).join('\n').replace(/<[^>]*>/g, '');
    return [{ start, end, text: decoder.value }];
  });
}
