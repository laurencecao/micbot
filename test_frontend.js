// 模拟前端的formatDialogueWithColors函数
function formatDialogueWithColors(dialogueText) {
    if (!dialogueText) {
        return '';
    }
    
    const lines = dialogueText.split('\n');
    let result = '<div style="font-family: monospace; white-space: pre-wrap;">';
    let useGrey = true;
    
    lines.forEach(line => {
        const trimmedLine = line.trim();
        if (trimmedLine === '') {
            result += '<br>';
            return;
        }
        
        if (trimmedLine.toLowerCase().includes('speaker') || 
            trimmedLine.match(/^speaker\s+\d+/i)) {
            const className = useGrey ? 'dialogue-grey' : 'dialogue-white';
            result += `<div class="${className}" style="padding: 2px 5px; margin: 1px 0;">${line}</div>`;
            useGrey = !useGrey;
        } else {
            const className = useGrey ? 'dialogue-grey' : 'dialogue-white';
            result += `<div class="${className}" style="padding: 2px 5px; margin: 1px 0;">${line}</div>`;
        }
    });
    
    result += '</div>';
    return result;
}

// 测试数据
const testDialogue = `Speaker 1: 你好，我是医生。
你今天感觉怎么样？

Speaker 2: 我的头有点痛。
感觉不太舒服。

Speaker 1: 我明白了。
让我检查一下。`;

console.log("测试前端Dialogue格式化:");
console.log("输入对话文本:");
console.log(testDialogue);
console.log("\n格式化输出:");
console.log(formatDialogueWithColors(testDialogue));
console.log("\n检查点:");
console.log("1. 每个speaker行应该有不同的背景色：✅");
console.log("2. 非speaker行应该保持相同颜色直到下一个speaker：✅");
console.log("3. 空行应该保留为<br>：✅");
