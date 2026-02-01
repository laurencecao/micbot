let mediaRecorder = null;
let audioChunks = [];
let currentRecordId = null;
let currentAction = null;
let currentAudio = null;
let currentPlayBtn = null;

const startBtn = document.getElementById('startBtn');
const stopBtn = document.getElementById('stopBtn');
const recordingIndicator = document.getElementById('recordingIndicator');
const recordingStatus = document.getElementById('recordingStatus');
const progressBar = document.getElementById('progressBar');
const progressFill = document.getElementById('progressFill');
const progressText = document.getElementById('progressText');
const recordsTable = document.getElementById('recordsTable');
const textModal = document.getElementById('textModal');
const textInput = document.getElementById('textInput');
const modalTitle = document.getElementById('modalTitle');

document.addEventListener('DOMContentLoaded', () => {
    loadRecords();
});

startBtn.addEventListener('click', async () => {
    try {
        const stream = await navigator.mediaDevices.getUserMedia({ audio: true });
        mediaRecorder = new MediaRecorder(stream);
        audioChunks = [];

        mediaRecorder.ondataavailable = (event) => {
            if (event.data.size > 0) {
                audioChunks.push(event.data);
            }
        };

        mediaRecorder.onstop = async () => {
            const audioBlob = new Blob(audioChunks, { type: 'audio/webm' });
            await uploadAudio(audioBlob);
            
            stream.getTracks().forEach(track => track.stop());
        };

        mediaRecorder.start();
        
        startBtn.disabled = true;
        stopBtn.disabled = false;
        recordingIndicator.classList.add('active');
        recordingStatus.textContent = '正在录音...';
        
    } catch (err) {
        alert('无法访问麦克风: ' + err.message);
        console.error('Error accessing microphone:', err);
    }
});

stopBtn.addEventListener('click', () => {
    if (mediaRecorder && mediaRecorder.state !== 'inactive') {
        mediaRecorder.stop();
        
        startBtn.disabled = false;
        stopBtn.disabled = true;
        recordingIndicator.classList.remove('active');
        recordingStatus.textContent = '准备就绪';
    }
});

async function uploadAudio(audioBlob) {
    const formData = new FormData();
    formData.append('audio', audioBlob, 'recording.webm');
    
    progressBar.classList.add('active');
    progressFill.style.width = '0%';
    progressText.textContent = '上传中... 0%';
    
    try {
        const xhr = new XMLHttpRequest();
        
        xhr.upload.addEventListener('progress', (event) => {
            if (event.lengthComputable) {
                const percentComplete = Math.round((event.loaded / event.total) * 100);
                progressFill.style.width = percentComplete + '%';
                progressText.textContent = `上传中... ${percentComplete}%`;
            }
        });
        
        xhr.addEventListener('load', () => {
            if (xhr.status === 200) {
                const response = JSON.parse(xhr.responseText);
                if (response.success) {
                    progressFill.style.width = '100%';
                    progressText.textContent = '上传完成！';
                    
                    setTimeout(() => {
                        progressBar.classList.remove('active');
                        progressText.textContent = '';
                        loadRecords();
                    }, 1000);
                }
            } else {
                let errorMsg = '上传失败';
                try {
                    const errorResponse = JSON.parse(xhr.responseText);
                    errorMsg = errorResponse.message || '上传失败';
                } catch (e) {
                    errorMsg = `上传失败 (${xhr.status})`;
                }
                progressText.textContent = errorMsg;
                console.error('Upload failed:', xhr.status, xhr.responseText);
                setTimeout(() => {
                    progressBar.classList.remove('active');
                    progressText.textContent = '';
                }, 3000);
            }
        });
        
        xhr.addEventListener('error', () => {
            progressText.textContent = '上传出错，请检查网络连接';
            console.error('Upload error: Network error');
            setTimeout(() => {
                progressBar.classList.remove('active');
                progressText.textContent = '';
            }, 3000);
        });
        
        xhr.open('POST', '/api/mobile/records/upload');
        xhr.send(formData);
        
    } catch (err) {
        console.error('Upload error:', err);
        progressText.textContent = '上传失败';
        setTimeout(() => {
            progressBar.classList.remove('active');
            progressText.textContent = '';
        }, 2000);
    }
}

async function loadRecords() {
    try {
        const response = await fetch('/api/mobile/records');
        const records = await response.json();

        if (records.length === 0) {
            recordsTable.innerHTML = '<tr><td colspan="5" class="empty-state">暂无记录</td></tr>';
            return;
        }

        recordsTable.innerHTML = records.map(record => {
            const audioTextId = `audio-text-${record.id}`;
            const hisRecordId = `his-record-${record.id}`;
            return `
            <tr>
                <td>${record.id}</td>
                <td>
                    <div class="cell-content">${record.diagnosis_record || '(空)'}</div>
                    <div class="action-btns">
                        <button class="btn btn-small btn-primary" onclick="openTextModal(${record.id}, 'diagnosis')">上传诊疗记录</button>
                    </div>
                </td>
                <td>
                    ${record.audio_file ? `
                        <div class="audio-player">
                            <button class="play-btn" onclick="togglePlay(this, '/mobile/audio/${record.audio_file}')" title="点击播放">▶</button>
                            <span class="audio-file-name">${record.audio_file}</span>
                        </div>
                    ` : '(无录音)'}
                </td>
                <td>
                    <div class="cell-content" id="${audioTextId}"></div>
                    <div class="action-btns">
                        <button class="btn btn-small btn-success" onclick="convertToText(${record.id}, this)">转为文本</button>
                    </div>
                </td>
                <td>
                    <div class="cell-content" id="${hisRecordId}">${record.his_record || '(空)'}</div>
                </td>
            </tr>
        `}).join('');

        records.forEach(record => {
            const audioTextElement = document.getElementById(`audio-text-${record.id}`);
            if (audioTextElement) {
                audioTextElement.innerHTML = record.audio_text || '(空)';
            }

            // 处理 HIS 记录显示
            const hisRecordElement = document.getElementById(`his-record-${record.id}`);
            if (hisRecordElement && record.his_record && record.his_record !== '(空)') {
                try {
                    // 尝试解析 JSON
                    const hisData = JSON.parse(record.his_record);
                    hisRecordElement.innerHTML = renderHisTable(hisData);
                } catch (e) {
                    // 如果不是 JSON，直接显示文本
                    hisRecordElement.innerHTML = record.his_record;
                }
            }

            // 检查是否需要自动生成 HIS 记录
            // 条件：录音听写非空，且整理后的HIS记录为空
            if (record.audio_text && !record.his_record) {
                generateHisRecord(record.id, record.audio_text, record.diagnosis_record || '');
            }
        });

    } catch (err) {
        console.error('Error loading records:', err);
        recordsTable.innerHTML = '<tr><td colspan="5" class="empty-state">加载失败，请刷新重试</td></tr>';
    }
}

// 生成 HIS 记录并显示为表格
async function generateHisRecord(recordId, dialogue, history) {
    const hisRecordElement = document.getElementById(`his-record-${recordId}`);
    if (!hisRecordElement) return;

    // 显示生成中状态
    hisRecordElement.innerHTML = '<div style="color: #1890ff;">正在生成 HIS 记录...</div>';

    try {
        const response = await fetch('/api/mobile/records/generate-his', {
            method: 'POST',
            headers: {
                'Content-Type': 'application/json'
            },
            body: JSON.stringify({
                id: recordId,
                dialogue: dialogue,
                history: history
            })
        });

        const result = await response.json();

        if (result.status === 'success' && result.table_data) {
            // 渲染表格（后端已自动保存到数据库）
            hisRecordElement.innerHTML = renderHisTable(result.table_data);
        } else {
            hisRecordElement.innerHTML = '<div style="color: #ff4d4f;">生成失败: ' + (result.detail || '未知错误') + '</div>';
        }
    } catch (err) {
        console.error('Error generating HIS record:', err);
        hisRecordElement.innerHTML = '<div style="color: #ff4d4f;">生成失败，请重试</div>';
    }
}

// 保存 HIS 记录到数据库
async function saveHisRecord(recordId, hisRecord) {
    try {
        const response = await fetch('/api/mobile/records/update-his', {
            method: 'POST',
            headers: {
                'Content-Type': 'application/json'
            },
            body: JSON.stringify({
                id: recordId,
                his_record: hisRecord
            })
        });

        const result = await response.json();

        if (result.success) {
            console.log('HIS record saved successfully for record:', recordId);
        } else {
            console.error('Failed to save HIS record:', result.message);
        }
    } catch (err) {
        console.error('Error saving HIS record:', err);
    }
}

// 将 HIS 数据渲染为 HTML 表格（隔行底色）
function renderHisTable(tableData) {
    const fields = [
        '主诉',
        '现病史',
        '既往史',
        '药物过敏史',
        '体格检查',
        '辅助检查',
        '诊断',
        '处理',
        '注意事项',
        '健康宣教',
        '医师签名'
    ];

    let rows = '';
    fields.forEach((field, index) => {
        const value = tableData[field] || '暂无';
        const rowClass = index % 2 === 0 ? 'his-row-even' : 'his-row-odd';
        rows += `
            <tr class="${rowClass}">
                <td class="his-field-name">${field}</td>
                <td class="his-field-value">${value.replace(/\n/g, '<br>')}</td>
            </tr>
        `;
    });

    return `
        <table class="his-record-table">
            <tbody>
                ${rows}
            </tbody>
        </table>
    `;
}

function openTextModal(id, type) {
    currentRecordId = id;
    currentAction = type;
    
    if (type === 'diagnosis') {
        modalTitle.textContent = '上传诊疗记录';
        textInput.placeholder = '请输入诊疗记录内容...';
    }
    
    textInput.value = '';
    textModal.classList.add('active');
}

function closeModal() {
    textModal.classList.remove('active');
    currentRecordId = null;
    currentAction = null;
}

async function saveText() {
    const content = textInput.value.trim();
    if (!content) {
        alert('请输入内容');
        return;
    }
    
    try {
        const response = await fetch('/api/mobile/records/update-diagnosis', {
            method: 'POST',
            headers: {
                'Content-Type': 'application/json'
            },
            body: JSON.stringify({
                id: currentRecordId,
                content: content
            })
        });
        
        const result = await response.json();
        
        if (result.success) {
            closeModal();
            loadRecords();
        } else {
            alert('保存失败: ' + result.message);
        }
    } catch (err) {
        console.error('Error saving text:', err);
        alert('保存失败，请重试');
    }
}

async function convertToText(id, button) {
    button.disabled = true;
    button.textContent = '转录中...';
    
    try {
        const response = await fetch('/api/mobile/records/transcribe', {
            method: 'POST',
            headers: {
                'Content-Type': 'application/json'
            },
            body: JSON.stringify({ id: id })
        });
        
        const result = await response.json();
        
        if (result.success) {
            loadRecords();
        } else {
            alert('转录失败: ' + (result.message || '未知错误'));
            button.disabled = false;
            button.textContent = '转为文本';
        }
    } catch (err) {
        console.error('Transcription error:', err);
        alert('转录失败，请重试');
        button.disabled = false;
        button.textContent = '转为文本';
    }
}

textModal.addEventListener('click', (e) => {
    if (e.target === textModal) {
        closeModal();
    }
});

function loadFileContent(event) {
    const file = event.target.files[0];
    if (!file) {
        return;
    }
    
    const reader = new FileReader();
    reader.onload = function(e) {
        const content = e.target.result;
        textInput.value = content;
    };
    reader.onerror = function(e) {
        alert('文件读取失败，请重试');
        console.error('File read error:', e);
    };
    reader.readAsText(file);
    
    event.target.value = '';
}

function togglePlay(btn, audioSrc) {
    if (currentAudio && currentPlayBtn === btn) {
        if (currentAudio.paused) {
            currentAudio.play();
            btn.innerHTML = '❚❚';
            btn.classList.add('playing');
        } else {
            currentAudio.pause();
            btn.innerHTML = '▶';
            btn.classList.remove('playing');
        }
        return;
    }
    
    if (currentAudio) {
        currentAudio.pause();
        currentAudio.currentTime = 0;
        if (currentPlayBtn) {
            currentPlayBtn.innerHTML = '▶';
            currentPlayBtn.classList.remove('playing');
        }
    }
    
    currentAudio = new Audio(audioSrc);
    currentPlayBtn = btn;
    
    currentAudio.addEventListener('ended', () => {
        btn.innerHTML = '▶';
        btn.classList.remove('playing');
        currentAudio = null;
        currentPlayBtn = null;
    });
    
    currentAudio.addEventListener('error', (e) => {
        console.error('Audio playback error:', e);
        alert('音频播放失败，请检查文件格式');
        btn.innerHTML = '▶';
        btn.classList.remove('playing');
        currentAudio = null;
        currentPlayBtn = null;
    });
    
    const playPromise = currentAudio.play();
    
    if (playPromise !== undefined) {
        playPromise.then(() => {
            btn.innerHTML = '❚❚';
            btn.classList.add('playing');
        }).catch(err => {
            console.error('Play error:', err);
            alert('无法播放音频: ' + err.message);
            btn.innerHTML = '▶';
            btn.classList.remove('playing');
            currentAudio = null;
            currentPlayBtn = null;
        });
    }
}