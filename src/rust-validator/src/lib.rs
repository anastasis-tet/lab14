use chrono::DateTime;
use pyo3::exceptions::PyValueError;
use pyo3::prelude::*;

#[pyfunction]
fn validate_aggregate(
    category: String,
    count: i64,
    min_latitude: f64,
    max_latitude: f64,
    avg_latitude: f64,
    window_start: String,
    window_end: String,
) -> PyResult<()> {
    if category.trim().is_empty() {
        return Err(PyValueError::new_err("category is required"));
    }
    if count < 0 {
        return Err(PyValueError::new_err("count must be non-negative"));
    }
    for value in [min_latitude, max_latitude, avg_latitude] {
        if !(-90.0..=90.0).contains(&value) {
            return Err(PyValueError::new_err("latitude must be in range -90..90"));
        }
    }
    if min_latitude > max_latitude {
        return Err(PyValueError::new_err("min_latitude cannot exceed max_latitude"));
    }
    if avg_latitude < min_latitude || avg_latitude > max_latitude {
        return Err(PyValueError::new_err("avg_latitude must be between min and max"));
    }

    let start = DateTime::parse_from_rfc3339(&window_start)
        .map_err(|_| PyValueError::new_err("window_start must be RFC3339"))?;
    let end = DateTime::parse_from_rfc3339(&window_end)
        .map_err(|_| PyValueError::new_err("window_end must be RFC3339"))?;
    if end <= start {
        return Err(PyValueError::new_err("window_end must be greater than window_start"));
    }
    Ok(())
}

#[pymodule]
fn climate_validator(module: &Bound<'_, PyModule>) -> PyResult<()> {
    module.add_function(wrap_pyfunction!(validate_aggregate, module)?)?;
    Ok(())
}

