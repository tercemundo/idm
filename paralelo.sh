#!/bin/bash

# Validar argumentos
if [ "$#" -ne 2 ]; then
    echo "Uso: $0 start_id end_id"
    exit 1
fi

start_id=$1
end_id=$2
num_processes=10
threads_per_process=50

# Calcular el rango total y el tamaño de cada segmento
total_range=$((end_id - start_id))
segment_size=$((total_range / num_processes))

# Verificar que el ejecutable existe
if [ ! -f "./vimeo-scraper" ]; then
    echo "Error: No se encuentra el ejecutable vimeo-scraper"
    exit 1
fi

# Crear directorio para logs si no existe
mkdir -p logs

# Función para iniciar un proceso
start_process() {
    local process_num=$1
    local start=$2
    local end=$3
    
    echo "Iniciando proceso $process_num - Rango: $start a $end"
    
    nohup ./vimeo-scraper \
        --start $start \
        --end $end \
        --threads $threads_per_process \
        --output "resultados_${process_num}.txt" \
        > "logs/script${process_num}_output.txt" 2>&1 &
    
    echo "PID del proceso $process_num: $!"
    echo "$!" >> logs/pids.txt
}

# Limpiar archivo de PIDs si existe
echo -n > logs/pids.txt

# Iniciar los procesos
for ((i=1; i<=num_processes; i++)); do
    process_start=$((start_id + (i-1)*segment_size))
    
    # Para el último proceso, asegurarse de llegar hasta el end_id
    if [ $i -eq $num_processes ]; then
        process_end=$end_id
    else
        process_end=$((start_id + i*segment_size - 1))
    fi
    
    start_process $i $process_start $process_end
done

echo "Todos los procesos han sido iniciados"
echo "Los PIDs están guardados en logs/pids.txt"
echo "Los logs están en el directorio logs/"

# Crear script para detener todos los procesos
cat > stop_all.sh << 'EOF'
#!/bin/bash
if [ -f logs/pids.txt ]; then
    while read pid; do
        if kill -0 $pid 2>/dev/null; then
            echo "Deteniendo proceso $pid"
            kill $pid
        fi
    done < logs/pids.txt
    echo "Todos los procesos han sido detenidos"
else
    echo "No se encuentra el archivo de PIDs"
fi
EOF

chmod +x stop_all.sh

echo "Se ha creado stop_all.sh para detener todos los procesos cuando sea necesario"

# Crear script para verificar el estado
cat > check_status.sh << 'EOF'
#!/bin/bash
echo "Estado de los procesos:"
echo "----------------------"
if [ -f logs/pids.txt ]; then
    while read pid; do
        if kill -0 $pid 2>/dev/null; then
            echo "Proceso $pid: RUNNING"
            echo "Últimas líneas del log:"
            process_num=$(ps -p $pid -o cmd= | grep -o 'resultados_[0-9]\+\.txt' | cut -d'_' -f2 | cut -d'.' -f1)
            tail -n 3 "logs/script${process_num}_output.txt"
            echo "----------------------"
        else
            echo "Proceso $pid: STOPPED"
        fi
    done < logs/pids.txt
else
    echo "No se encuentra el archivo de PIDs"
fi
EOF

chmod +x check_status.sh

echo "Se ha creado check_status.sh para verificar el estado de los procesos"
Last
