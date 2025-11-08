"use strict";

const recordTypeFields = {
    'A': [
        { id: 'ipv4', label: 'IPv4 Address', type: 'text', required: true }
    ],
    'CNAME': [
        { id: 'cname', label: 'Target Domain', type: 'text', required: true }
    ],
    'TXT': [
        { id: 'txt', label: 'Text Value', type: 'textarea', required: true }
    ],
    'SRV': [
        { id: 'priority', label: 'Priority', type: 'number', required: true },
        { id: 'weight', label: 'Weight', type: 'number', required: true },
        { id: 'port', label: 'Port', type: 'number', required: true },
        { id: 'target', label: 'Target', type: 'text', required: true }
    ]
};

// Load DNS records when the page loads
document.addEventListener('DOMContentLoaded', () => {
    loadDnsRecords();
    setupFormHandlers();
});

async function loadDnsRecords() {
    try {
        const response = await fetch('/api/dns');
        const records = await response.json();
        displayRecords(records);
    } catch (error) {
        console.error('Error loading DNS records:', error);
    }
}

function displayRecords(records) {
    const containers = {
        'A': document.getElementById('a-records'),
        'CNAME': document.getElementById('cname-records'),
        'TXT': document.getElementById('txt-records'),
        'SRV': document.getElementById('srv-records')
    };
    
    Object.values(containers).forEach(container => {
        if (container) container.innerHTML = '';
    });

    records.forEach(record => {
        const container = containers[record.type];
        if (!container) return;

        const recordElement = createRecordElement(record);
        container.appendChild(recordElement);
    });
}

function createRecordElement(record) {
    const div = document.createElement('div');
    div.className = 'record-card';
    
    let content = '';
    switch (record.type) {
        case 'A':
            content = record.ipv4;
            break;
        case 'CNAME':
            content = record.cname;
            break;
        case 'TXT':
            content = record.txt ? record.txt.join(', ') : '';
            break;
        case 'SRV':
            content = `Priority: ${record.priority}, Weight: ${record.weight}, Port: ${record.port}, Target: ${record.target}`;
            break;
    }

    div.innerHTML = `
        <div class="record-header">
            <span class="record-name">${record.name}</span>
            <div class="record-actions">
                <button class="button secondary" onclick="editRecord(${record.id})">Edit</button>
                <button class="button danger" onclick="deleteRecord(${record.id})">Delete</button>
            </div>
        </div>
        <div class="record-content">
            <div>TTL: ${record.ttl}s</div>
            <div>${content}</div>
        </div>
    `;

    return div;
}

function showAddRecordModal() {
    const modal = document.getElementById('add-record-modal');
    modal.style.display = 'flex';
    updateFormFields();
}

function hideAddRecordModal() {
    const modal = document.getElementById('add-record-modal');
    modal.style.display = 'none';
}

function showEditRecordModal() {
    const modal = document.getElementById('edit-record-modal');
    modal.style.display = 'flex';
}

function hideEditRecordModal() {
    const modal = document.getElementById('edit-record-modal');
    modal.style.display = 'none';
}

function updateFormFields() {
    const type = document.getElementById('recordType').value;
    const container = document.getElementById('dynamic-fields');
    container.innerHTML = '';

    recordTypeFields[type].forEach(field => {
        const div = document.createElement('div');
        div.className = 'form-group';
        
        const label = document.createElement('label');
        label.htmlFor = field.id;
        label.textContent = field.label;
        
        const input = document.createElement(field.type === 'textarea' ? 'textarea' : 'input');
        input.id = field.id;
        if (field.type !== 'textarea') {
            input.type = field.type;
        }
        input.required = field.required;

        div.appendChild(label);
        div.appendChild(input);
        container.appendChild(div);
    });
}

function setupFormHandlers() {
    document.getElementById('add-record-form').addEventListener('submit', async (e) => {
        e.preventDefault();
        
        const type = document.getElementById('recordType').value;
        const record = {
            name: document.getElementById('name').value,
            type: type,
            ttl: parseInt(document.getElementById('ttl').value),
            class: 'IN'
        };

        recordTypeFields[type].forEach(field => {
            const value = document.getElementById(field.id).value;
            if (field.type === 'number') {
                record[field.id] = parseInt(value);
            } else if (field.id === 'txt') {
                record[field.id] = [value];
            } else {
                record[field.id] = value;
            }
        });

        try {
            const response = await fetch('/api/dns', {
                method: 'POST',
                headers: {
                    'Content-Type': 'application/json'
                },
                body: JSON.stringify(record)
            });

            if (response.ok) {
                hideAddRecordModal();
                loadDnsRecords();
            } else {
                const error = await response.json();
                alert('Error creating record: ' + error.error);
            }
        } catch (error) {
            console.error('Error creating record:', error);
            alert('Error creating record');
        }
    });

    document.getElementById('edit-record-form').addEventListener('submit', async (e) => {
        e.preventDefault();
        
        const id = document.getElementById('edit-record-id').value;
        const record = {
            name: document.getElementById('edit-name').value,
            ttl: parseInt(document.getElementById('edit-ttl').value)
        };

        try {
            const response = await fetch(`/api/dns/${id}`, {
                method: 'PUT',
                headers: {
                    'Content-Type': 'application/json'
                },
                body: JSON.stringify(record)
            });

            if (response.ok) {
                hideEditRecordModal();
                loadDnsRecords();
            } else {
                const error = await response.json();
                alert('Error updating record: ' + error.error);
            }
        } catch (error) {
            console.error('Error updating record:', error);
            alert('Error updating record');
        }
    });
}

async function editRecord(id) {
    try {
        const response = await fetch(`/api/dns/id/${id}`);
        const record = await response.json();
        
        document.getElementById('edit-record-id').value = record.id;
        document.getElementById('edit-name').value = record.name;
        document.getElementById('edit-ttl').value = record.ttl;
        
        const container = document.getElementById('edit-dynamic-fields');
        container.innerHTML = '';
        
        recordTypeFields[record.type].forEach(field => {
            const div = document.createElement('div');
            div.className = 'form-group';
            
            const label = document.createElement('label');
            label.htmlFor = 'edit-' + field.id;
            label.textContent = field.label;
            
            const input = document.createElement(field.type === 'textarea' ? 'textarea' : 'input');
            input.id = 'edit-' + field.id;
            if (field.type !== 'textarea') {
                input.type = field.type;
            }
            input.required = field.required;
            input.value = record[field.id];

            div.appendChild(label);
            div.appendChild(input);
            container.appendChild(div);
        });

        showEditRecordModal();
    } catch (error) {
        console.error('Error loading record for editing:', error);
        alert('Error loading record');
    }
}

async function deleteRecord(id) {
    if (!confirm('Are you sure you want to delete this record?')) {
        return;
    }

    try {
        const response = await fetch(`/api/dns/${id}`, {
            method: 'DELETE'
        });

        if (response.ok) {
            loadDnsRecords();
        } else {
            const error = await response.json();
            alert('Error deleting record: ' + error.error);
        }
    } catch (error) {
        console.error('Error deleting record:', error);
        alert('Error deleting record');
    }
}